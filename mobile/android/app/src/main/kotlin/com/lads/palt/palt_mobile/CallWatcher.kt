package com.lads.palt.palt_mobile

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.media.AudioManager
import android.net.Uri
import android.provider.ContactsContract
import android.telephony.TelephonyManager
import android.util.Base64
import java.io.ByteArrayOutputStream

/**
 * Watches calls so paired computers can show who's calling, pause media
 * and note missed calls. Needs READ_PHONE_STATE and READ_CALL_LOG (for the
 * number), and READ_CONTACTS for the name and photo.
 *
 * Events: "ringing", "talking", "missed", "ended" with {number, name,
 * photo?, direction (incoming|outgoing), since (ms the call was picked up
 * or dialled, 0 while ringing)}.
 */
object CallWatcher {
    private var state = TelephonyManager.EXTRA_STATE_IDLE
    private var number = ""
    private var muted = false
    private var direction = "incoming"
    private var since = 0L
    private var dialled = ""

    /** PhoneCalls is about to dial: Android doesn't tell apps the number. */
    fun outgoing(n: String) {
        dialled = n
    }

    /** The call in progress, for computers that link up mid-call; null if none. */
    fun current(context: Context): Map<String, Any?>? {
        if (state == TelephonyManager.EXTRA_STATE_IDLE) return null
        return describe(context, if (state == TelephonyManager.EXTRA_STATE_RINGING) "ringing" else "talking")
    }

    var sink: ((Map<String, Any?>) -> Unit)? = null

    fun start(context: Context) {
        val app = context.applicationContext
        app.registerReceiver(object : BroadcastReceiver() {
            override fun onReceive(c: Context, i: Intent) {
                val s = i.getStringExtra(TelephonyManager.EXTRA_STATE) ?: return
                // The number comes in a second broadcast once READ_CALL_LOG is granted.
                @Suppress("DEPRECATION")
                val n = i.getStringExtra(TelephonyManager.EXTRA_INCOMING_NUMBER)
                if (!n.isNullOrEmpty()) number = n
                if (s == state && n.isNullOrEmpty()) return
                val prev = state
                state = s
                when (s) {
                    TelephonyManager.EXTRA_STATE_RINGING -> {
                        direction = "incoming"
                        since = 0L
                    }
                    TelephonyManager.EXTRA_STATE_OFFHOOK -> if (prev != s) {
                        // From idle it's a call being dialled; from ringing, one answered.
                        if (prev == TelephonyManager.EXTRA_STATE_IDLE) {
                            direction = "outgoing"
                            if (number.isEmpty()) number = dialled
                        }
                        since = System.currentTimeMillis()
                    }
                }
                val event = when (s) {
                    // Without call-log access no number ever comes: announce it anyway.
                    TelephonyManager.EXTRA_STATE_RINGING ->
                        if (n.isNullOrEmpty() &&
                            c.checkSelfPermission(Manifest.permission.READ_CALL_LOG) == PackageManager.PERMISSION_GRANTED
                        ) return else "ringing"
                    TelephonyManager.EXTRA_STATE_OFFHOOK -> "talking"
                    else -> if (prev == TelephonyManager.EXTRA_STATE_RINGING) "missed" else "ended"
                }
                if (event == "ended" || event == "missed" || event == "talking") unmute(app)
                emit(app, event)
                if (s == TelephonyManager.EXTRA_STATE_IDLE) {
                    number = ""
                    dialled = ""
                    since = 0L
                }
            }
        }, IntentFilter(TelephonyManager.ACTION_PHONE_STATE_CHANGED))
    }

    private fun describe(context: Context, event: String): Map<String, Any?> {
        val (name, photo) = lookup(context, number)
        return mapOf(
            "event" to event, "number" to number, "name" to name, "photo" to photo,
            "direction" to direction, "since" to since,
        )
    }

    private fun emit(context: Context, event: String) {
        sink?.invoke(describe(context, event))
    }

    /** Silences the ringer for the current call (restored when it ends). */
    fun mute(context: Context) {
        if (muted || state != TelephonyManager.EXTRA_STATE_RINGING) return
        context.getSystemService(AudioManager::class.java)
            .adjustStreamVolume(AudioManager.STREAM_RING, AudioManager.ADJUST_MUTE, 0)
        muted = true
    }

    private fun unmute(context: Context) {
        if (!muted) return
        muted = false
        context.getSystemService(AudioManager::class.java)
            .adjustStreamVolume(AudioManager.STREAM_RING, AudioManager.ADJUST_UNMUTE, 0)
    }

    /** Contact name and photo (base64 JPEG) for a number, when allowed. */
    private fun lookup(context: Context, number: String): Pair<String, String?> {
        if (number.isEmpty() ||
            context.checkSelfPermission(Manifest.permission.READ_CONTACTS) != PackageManager.PERMISSION_GRANTED
        ) return Pair("", null)
        val uri = Uri.withAppendedPath(ContactsContract.PhoneLookup.CONTENT_FILTER_URI, Uri.encode(number))
        try {
            context.contentResolver.query(
                uri,
                arrayOf(ContactsContract.PhoneLookup.DISPLAY_NAME, ContactsContract.PhoneLookup.PHOTO_THUMBNAIL_URI),
                null, null, null
            )?.use { c ->
                if (c.moveToFirst()) {
                    val name = c.getString(0) ?: ""
                    val photo = c.getString(1)?.let { photoBase64(context, Uri.parse(it)) }
                    return Pair(name, photo)
                }
            }
        } catch (_: Exception) {
        }
        return Pair("", null)
    }

    private fun photoBase64(context: Context, uri: Uri): String? = try {
        context.contentResolver.openInputStream(uri)?.use { input ->
            val bmp = BitmapFactory.decodeStream(input) ?: return null
            val out = ByteArrayOutputStream()
            Bitmap.createScaledBitmap(bmp, 128, 128, true).compress(Bitmap.CompressFormat.JPEG, 85, out)
            Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP)
        }
    } catch (_: Exception) {
        null
    }
}
