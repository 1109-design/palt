package com.lads.palt.palt_mobile

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.CallLog
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import android.telephony.SubscriptionManager
import android.telephony.TelephonyManager

/**
 * Calls for computers (lib/features/calls.dart): the SIMs, the call log,
 * and placing, answering and ending calls through TelecomManager. The call
 * itself (audio) stays on the phone.
 */
object PhoneCalls {
    private fun allowed(context: Context, permission: String) =
        context.checkSelfPermission(permission) == PackageManager.PERMISSION_GRANTED

    /** Active SIMs: {id (subscription id), slot (1-based), name, number}. */
    fun sims(context: Context): List<Map<String, Any?>> {
        if (!allowed(context, Manifest.permission.READ_PHONE_STATE)) return emptyList()
        val sm = context.getSystemService(SubscriptionManager::class.java)
        return try {
            sm.activeSubscriptionInfoList.orEmpty().sortedBy { it.simSlotIndex }.map { s ->
                mapOf(
                    "id" to s.subscriptionId,
                    "slot" to s.simSlotIndex + 1,
                    "name" to (s.displayName?.toString()?.takeIf { it.isNotBlank() } ?: s.carrierName?.toString() ?: "SIM ${s.simSlotIndex + 1}"),
                    "carrier" to (s.carrierName?.toString() ?: ""),
                )
            }
        } catch (_: SecurityException) {
            emptyList()
        }
    }

    /** subscription id → 1-based slot, and phone account id → slot. */
    private fun slots(context: Context): Map<String, Int> {
        val out = HashMap<String, Int>()
        val sm = context.getSystemService(SubscriptionManager::class.java)
        try {
            for (s in sm.activeSubscriptionInfoList.orEmpty()) {
                out[s.subscriptionId.toString()] = s.simSlotIndex + 1
                @Suppress("DEPRECATION")
                s.iccId?.takeIf { it.isNotEmpty() }?.let { out[it] = s.simSlotIndex + 1 }
            }
        } catch (_: SecurityException) {
        }
        return out
    }

    /**
     * Recent calls, newest first, older than [before] (ms, 0 = now):
     * {number, name, type (incoming|outgoing|missed|rejected|blocked|voicemail),
     * date, duration (s), sim (slot, 0 unknown)}.
     */
    fun log(context: Context, before: Long, limit: Int): List<Map<String, Any?>> {
        if (!allowed(context, Manifest.permission.READ_CALL_LOG)) throw SecurityException("PALT isn't allowed to read the call log")
        val slots = slots(context)
        val out = ArrayList<Map<String, Any?>>()
        val where = if (before > 0) "${CallLog.Calls.DATE} < ?" else null
        val args = if (before > 0) arrayOf(before.toString()) else null
        context.contentResolver.query(
            CallLog.Calls.CONTENT_URI,
            arrayOf(
                CallLog.Calls.NUMBER, CallLog.Calls.CACHED_NAME, CallLog.Calls.TYPE, CallLog.Calls.DATE,
                CallLog.Calls.DURATION, CallLog.Calls.PHONE_ACCOUNT_ID,
            ),
            where, args, "${CallLog.Calls.DATE} DESC"
        )?.use { c ->
            while (c.moveToNext() && out.size < limit) {
                val number = c.getString(0) ?: ""
                out.add(
                    mapOf(
                        "number" to number,
                        "name" to (c.getString(1)?.takeIf { it.isNotBlank() } ?: Messages.contactName(context, number)),
                        "type" to when (c.getInt(2)) {
                            CallLog.Calls.INCOMING_TYPE -> "incoming"
                            CallLog.Calls.OUTGOING_TYPE -> "outgoing"
                            CallLog.Calls.MISSED_TYPE -> "missed"
                            CallLog.Calls.REJECTED_TYPE -> "rejected"
                            CallLog.Calls.BLOCKED_TYPE -> "blocked"
                            CallLog.Calls.VOICEMAIL_TYPE -> "voicemail"
                            else -> "incoming"
                        },
                        "date" to c.getLong(3),
                        "duration" to c.getLong(4),
                        "sim" to (slots[c.getString(5) ?: ""] ?: 0),
                    )
                )
            }
        }
        return out
    }

    private fun accountFor(context: Context, subscriptionId: Int): PhoneAccountHandle? {
        if (subscriptionId <= 0) return null
        val telecom = context.getSystemService(TelecomManager::class.java)
        val tm = context.getSystemService(TelephonyManager::class.java)
        return try {
            telecom.callCapablePhoneAccounts.firstOrNull { h ->
                (Build.VERSION.SDK_INT >= 30 && tm.getSubscriptionId(h) == subscriptionId) ||
                    h.id == subscriptionId.toString()
            }
        } catch (_: SecurityException) {
            null
        }
    }

    /** Dials [number] on the phone, on SIM [subscriptionId] (0 = default). */
    fun place(context: Context, number: String, subscriptionId: Int) {
        if (!allowed(context, Manifest.permission.CALL_PHONE)) throw SecurityException("PALT isn't allowed to make calls")
        val extras = Bundle()
        accountFor(context, subscriptionId)?.let { extras.putParcelable(TelecomManager.EXTRA_PHONE_ACCOUNT_HANDLE, it) }
        CallWatcher.outgoing(number)
        context.getSystemService(TelecomManager::class.java).placeCall(Uri.fromParts("tel", number, null), extras)
    }

    /** Answers the ringing call. */
    @Suppress("DEPRECATION")
    fun answer(context: Context) {
        if (!allowed(context, Manifest.permission.ANSWER_PHONE_CALLS)) throw SecurityException("PALT isn't allowed to answer calls")
        context.getSystemService(TelecomManager::class.java).acceptRingingCall()
    }

    /** Ends (or declines) the current call. */
    @Suppress("DEPRECATION")
    fun end(context: Context): Boolean {
        if (!allowed(context, Manifest.permission.ANSWER_PHONE_CALLS)) throw SecurityException("PALT isn't allowed to end calls")
        return context.getSystemService(TelecomManager::class.java).endCall()
    }
}
