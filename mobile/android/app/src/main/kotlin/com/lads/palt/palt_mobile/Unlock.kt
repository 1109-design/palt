package com.lads.palt.palt_mobile

import android.app.Activity
import android.app.KeyguardManager
import android.content.Context
import android.content.Intent
import android.hardware.biometrics.BiometricManager
import android.hardware.biometrics.BiometricPrompt
import android.os.Build
import android.os.Bundle
import android.os.CancellationSignal
import android.os.Handler
import android.os.Looper
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyPermanentlyInvalidatedException
import android.security.keystore.KeyProperties
import android.util.Base64
import android.util.Log
import android.annotation.TargetApi
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.PrivateKey
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap

/**
 * Unlocks paired computers with the phone's fingerprint (lib/features/unlock.dart).
 *
 * Each computer gets its own P-256 key in the Android Keystore that can only
 * sign right after a strong biometric, and stops working if fingerprints are
 * added or removed. The computer keeps the public key; to unlock, it sends a
 * one-time challenge and [UnlockActivity] shows the fingerprint prompt and
 * signs it. Needs Android 11 (per-use biometric keys).
 */
@TargetApi(Build.VERSION_CODES.R)
object Unlock {
    private const val TAG = "PaltUnlock"
    /** Every challenge starts with this (desktop/cmd/palt-unlock). */
    private const val PREFIX = "PALT-UNLOCK-1"
    private const val TIMEOUT_MS = 25_000L

    class Request(
        val computer: String,
        val name: String,
        val message: String,
        val done: (String?, String?) -> Unit,
    ) {
        @Volatile var finished = false
        var activity: Activity? = null

        fun finish(signature: String?, error: String?) {
            synchronized(this) {
                if (finished) return
                finished = true
            }
            pending.values.remove(this)
            done(signature, error)
            activity?.finish()
        }
    }

    private val pending = ConcurrentHashMap<String, Request>()
    private val main = Handler(Looper.getMainLooper())

    private fun alias(computer: String) = "palt_unlock_$computer"

    private fun keyStore() = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    /** Makes a new key for [computer] (replacing any old one); its public half, base64. */
    fun setup(context: Context, computer: String): String {
        val can = context.getSystemService(BiometricManager::class.java)
            ?.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG)
        if (can != BiometricManager.BIOMETRIC_SUCCESS) {
            throw IllegalStateException("Add a fingerprint to this phone first (Settings → Security)")
        }
        val spec = KeyGenParameterSpec.Builder(alias(computer), KeyProperties.PURPOSE_SIGN)
            .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
            .setDigests(KeyProperties.DIGEST_SHA256)
            .setUserAuthenticationRequired(true)
            .setUserAuthenticationParameters(0, KeyProperties.AUTH_BIOMETRIC_STRONG)
            .setInvalidatedByBiometricEnrollment(true)
            .build()
        val kpg = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
        kpg.initialize(spec)
        val pair = kpg.generateKeyPair()
        Log.i(TAG, "New unlock key for $computer")
        return Base64.encodeToString(pair.public.encoded, Base64.NO_WRAP)
    }

    fun forget(computer: String) {
        keyStore().deleteEntry(alias(computer))
    }

    /** Asks for a fingerprint and signs [message]; [done] gets (signature, error). */
    fun sign(context: Context, computer: String, name: String, message: String, done: (String?, String?) -> Unit) {
        val parts = message.split("\n")
        if (parts.size != 5 || parts[0] != PREFIX) {
            done(null, "bad challenge")
            return
        }
        if (!keyStore().containsAlias(alias(computer))) {
            done(null, "Set up unlock again on the computer")
            return
        }
        val id = UUID.randomUUID().toString()
        val req = Request(computer, name, message, done)
        pending[id] = req
        main.postDelayed({ req.finish(null, "timed out") }, TIMEOUT_MS)
        try {
            context.startActivity(
                Intent(context, UnlockActivity::class.java)
                    .putExtra("id", id)
                    .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_NO_ANIMATION)
            )
        } catch (e: Exception) {
            req.finish(null, "couldn't show the fingerprint prompt")
        }
    }

    internal fun take(id: String?): Request? = id?.let { pending[it] }

    internal fun signer(computer: String): Signature {
        val key = keyStore().getKey(alias(computer), null) as PrivateKey
        return Signature.getInstance("SHA256withECDSA").apply { initSign(key) }
    }

    /** What the prompt says, from the challenge: host, user and what for. */
    internal fun title(message: String): Pair<String, String> {
        val (_, host, user, service) = message.split("\n")
        val title = when (service) {
            "gdm-password" -> "Unlock $host"
            "polkit-1" -> "Allow an admin action on $host"
            "sudo" -> "Allow sudo on $host"
            "setup" -> "Set up fingerprint unlock for $host"
            else -> "Sign in to $service on $host"
        }
        return title to "as $user"
    }
}

/** Shows the fingerprint prompt for one unlock request, over the lock screen too. */
@TargetApi(Build.VERSION_CODES.R)
class UnlockActivity : Activity() {
    private var req: Unlock.Request? = null
    private val cancel = CancellationSignal()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        overridePendingTransition(0, 0)
        setShowWhenLocked(true)
        setTurnScreenOn(true)
        val r = Unlock.take(intent.getStringExtra("id"))
        if (r == null || r.finished) {
            finish()
            return
        }
        req = r
        r.activity = this
        val keyguard = getSystemService(KeyguardManager::class.java)
        if (keyguard.isKeyguardLocked) {
            // The prompt needs the phone unlocked first.
            keyguard.requestDismissKeyguard(this, object : KeyguardManager.KeyguardDismissCallback() {
                override fun onDismissSucceeded() = prompt(r)
                override fun onDismissCancelled() = r.finish(null, "cancelled")
                override fun onDismissError() = r.finish(null, "cancelled")
            })
        } else {
            window.decorView.post { prompt(r) }
        }
    }

    private fun prompt(r: Unlock.Request) {
        val signer = try {
            Unlock.signer(r.computer)
        } catch (e: KeyPermanentlyInvalidatedException) {
            Unlock.forget(r.computer)
            r.finish(null, "Fingerprints changed on the phone; set up unlock again")
            return
        } catch (e: Exception) {
            r.finish(null, "Set up unlock again on the computer")
            return
        }
        val (title, subtitle) = Unlock.title(r.message)
        BiometricPrompt.Builder(this)
            .setTitle(title)
            .setSubtitle(subtitle)
            .setDescription("From PALT on ${r.name}")
            .setAllowedAuthenticators(BiometricManager.Authenticators.BIOMETRIC_STRONG)
            .setNegativeButton("Use password", mainExecutor) { _, _ -> r.finish(null, "cancelled") }
            .build()
            .authenticate(
                BiometricPrompt.CryptoObject(signer),
                cancel,
                mainExecutor,
                object : BiometricPrompt.AuthenticationCallback() {
                    override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                        val sig = try {
                            result.cryptoObject!!.signature!!.run {
                                update(r.message.toByteArray())
                                Base64.encodeToString(sign(), Base64.NO_WRAP)
                            }
                        } catch (e: Exception) {
                            r.finish(null, "signing failed")
                            return
                        }
                        r.finish(sig, null)
                    }

                    override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                        r.finish(null, errString.toString())
                    }
                },
            )
    }

    override fun onDestroy() {
        super.onDestroy()
        req?.let {
            if (!it.finished) {
                cancel.cancel()
                it.finish(null, "cancelled")
            }
        }
    }
}
