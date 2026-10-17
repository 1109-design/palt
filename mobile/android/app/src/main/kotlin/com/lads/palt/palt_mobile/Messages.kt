package com.lads.palt.palt_mobile

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.database.ContentObserver
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.ContactsContract
import android.provider.Telephony
import android.telephony.SmsManager
import android.util.Base64
import java.io.ByteArrayOutputStream

/**
 * Contacts and SMS for computers (lib/features/messages.dart): reading
 * conversations and contacts, sending texts, and new texts as they arrive.
 * PALT isn't the default SMS app, so it reads the SMS store and sends
 * through SmsManager (Android records those as sent); it can't mark texts
 * read or delete them.
 */
object Messages {
    /** Set by Channels: a text arrived (map like [message]). */
    var onReceived: ((Map<String, Any?>) -> Unit)? = null

    /** Set by Channels: SMS store or contacts changed ("sms" / "contacts"). */
    var onChanged: ((String) -> Unit)? = null

    private val main = Handler(Looper.getMainLooper())
    private var started = false

    private fun allowed(context: Context, permission: String) =
        context.checkSelfPermission(permission) == PackageManager.PERMISSION_GRANTED

    fun canRead(context: Context) = allowed(context, Manifest.permission.READ_SMS)

    /** Follows new texts and changes; safe to call again once permissions come. */
    fun start(context: Context) {
        if (started || !canRead(context)) return
        started = true
        val app = context.applicationContext
        val received = object : BroadcastReceiver() {
            override fun onReceive(c: Context, intent: Intent) {
                val parts = Telephony.Sms.Intents.getMessagesFromIntent(intent) ?: return
                if (parts.isEmpty()) return
                val body = parts.joinToString("") { it.messageBody ?: "" }
                val address = parts[0].originatingAddress ?: ""
                // The store isn't written yet; describe it from the PDUs.
                onReceived?.invoke(
                    mapOf(
                        "id" to 0L,
                        "threadId" to threadId(app, address),
                        "address" to address,
                        "body" to body,
                        "date" to parts[0].timestampMillis,
                        "type" to 1,
                        "read" to false,
                        "name" to contactName(app, address),
                    )
                )
            }
        }
        val filter = IntentFilter(Telephony.Sms.Intents.SMS_RECEIVED_ACTION)
        if (Build.VERSION.SDK_INT >= 33) {
            app.registerReceiver(received, filter, Context.RECEIVER_EXPORTED)
        } else {
            app.registerReceiver(received, filter)
        }
        var pending: Runnable? = null
        fun later(what: String) {
            pending?.let { main.removeCallbacks(it) }
            pending = Runnable { onChanged?.invoke(what) }.also { main.postDelayed(it, 800) }
        }
        app.contentResolver.registerContentObserver(Telephony.Sms.CONTENT_URI, true, object : ContentObserver(main) {
            override fun onChange(selfChange: Boolean) = later("sms")
        })
        if (allowed(app, Manifest.permission.READ_CONTACTS)) {
            app.contentResolver.registerContentObserver(
                ContactsContract.Contacts.CONTENT_URI, true,
                object : ContentObserver(main) {
                    override fun onChange(selfChange: Boolean) {
                        onChanged?.invoke("contacts")
                    }
                }
            )
        }
    }

    // ── SMS ──────────────────────────────────────────────────────────────────

    private val smsColumns = arrayOf(
        Telephony.Sms._ID, Telephony.Sms.THREAD_ID, Telephony.Sms.ADDRESS, Telephony.Sms.BODY,
        Telephony.Sms.DATE, Telephony.Sms.TYPE, Telephony.Sms.READ, Telephony.Sms.SUBSCRIPTION_ID,
    )

    private fun message(c: android.database.Cursor): Map<String, Any?> = mapOf(
        "id" to c.getLong(0),
        "threadId" to c.getLong(1),
        "address" to (c.getString(2) ?: ""),
        "body" to (c.getString(3) ?: ""),
        "date" to c.getLong(4),
        // 1 received; 2 sent; 4 outbox, 5 failed, 6 queued count as sent
        "type" to c.getInt(5),
        "read" to (c.getInt(6) != 0),
        "subscription" to c.getInt(7), // SIM subscription id, -1 unknown
    )

    /** Newest message of each conversation, newest first, with unread counts. */
    fun conversations(context: Context, limit: Int): List<Map<String, Any?>> {
        if (!canRead(context)) return emptyList()
        val latest = LinkedHashMap<Long, Map<String, Any?>>()
        val unread = HashMap<Long, Int>()
        context.contentResolver.query(
            Telephony.Sms.CONTENT_URI, smsColumns, null, null, "${Telephony.Sms.DATE} DESC"
        )?.use { c ->
            while (c.moveToNext()) {
                val thread = c.getLong(1)
                if (c.getInt(5) == 1 && c.getInt(6) == 0) unread[thread] = (unread[thread] ?: 0) + 1
                if (!latest.containsKey(thread) && latest.size < limit) latest[thread] = message(c)
            }
        }
        val names = HashMap<String, String>()
        return latest.values.map { m ->
            val address = m["address"] as String
            m + mapOf(
                "unread" to (unread[m["threadId"] as Long] ?: 0),
                "name" to names.getOrPut(address) { contactName(context, address) },
            )
        }
    }

    /** One conversation's messages, newest first, older than [before] (ms, 0 = now). */
    fun thread(context: Context, threadId: Long, before: Long, limit: Int): List<Map<String, Any?>> {
        if (!canRead(context)) return emptyList()
        val out = ArrayList<Map<String, Any?>>()
        val where = "${Telephony.Sms.THREAD_ID} = ?" + if (before > 0) " AND ${Telephony.Sms.DATE} < ?" else ""
        val args = if (before > 0) arrayOf(threadId.toString(), before.toString()) else arrayOf(threadId.toString())
        context.contentResolver.query(
            Telephony.Sms.CONTENT_URI, smsColumns, where, args, "${Telephony.Sms.DATE} DESC"
        )?.use { c ->
            while (c.moveToNext() && out.size < limit) out.add(message(c))
        }
        return out
    }

    private fun threadId(context: Context, address: String): Long = try {
        Telephony.Threads.getOrCreateThreadId(context, address)
    } catch (_: Exception) {
        0L
    }

    /** Sends a text from SIM [subscriptionId] (0 = default); throws with a reason when it can't. */
    fun send(context: Context, address: String, body: String, subscriptionId: Int) {
        if (!allowed(context, Manifest.permission.SEND_SMS)) throw SecurityException("PALT isn't allowed to send SMS")
        val sms = if (Build.VERSION.SDK_INT >= 31) {
            val base = context.getSystemService(SmsManager::class.java)
            if (subscriptionId > 0) base.createForSubscriptionId(subscriptionId) else base
        } else {
            @Suppress("DEPRECATION")
            if (subscriptionId > 0) SmsManager.getSmsManagerForSubscriptionId(subscriptionId) else SmsManager.getDefault()
        }
        val parts = sms.divideMessage(body)
        if (parts.size == 1) {
            sms.sendTextMessage(address, null, body, null, null)
        } else {
            sms.sendMultipartTextMessage(address, null, parts, null, null)
        }
    }

    // ── Contacts ─────────────────────────────────────────────────────────────

    fun contactName(context: Context, number: String): String {
        if (number.isEmpty() || !allowed(context, Manifest.permission.READ_CONTACTS)) return ""
        return try {
            context.contentResolver.query(
                Uri.withAppendedPath(ContactsContract.PhoneLookup.CONTENT_FILTER_URI, Uri.encode(number)),
                arrayOf(ContactsContract.PhoneLookup.DISPLAY_NAME), null, null, null
            )?.use { c -> if (c.moveToFirst()) c.getString(0) ?: "" else "" } ?: ""
        } catch (_: Exception) {
            ""
        }
    }

    /**
     * Every contact with a phone number: {id, name, numbers, photo (small
     * JPEG data URL or "")}, and a version that changes when any does.
     */
    fun contacts(context: Context): Map<String, Any> {
        if (!allowed(context, Manifest.permission.READ_CONTACTS)) return mapOf("version" to "", "contacts" to emptyList<Any>())
        data class C(val name: String, val photo: String?, val numbers: MutableList<String>, val updated: Long)
        val byId = LinkedHashMap<Long, C>()
        context.contentResolver.query(
            ContactsContract.CommonDataKinds.Phone.CONTENT_URI,
            arrayOf(
                ContactsContract.CommonDataKinds.Phone.CONTACT_ID,
                ContactsContract.CommonDataKinds.Phone.DISPLAY_NAME,
                ContactsContract.CommonDataKinds.Phone.NUMBER,
                ContactsContract.CommonDataKinds.Phone.PHOTO_THUMBNAIL_URI,
                ContactsContract.CommonDataKinds.Phone.CONTACT_LAST_UPDATED_TIMESTAMP,
            ),
            null, null, "${ContactsContract.CommonDataKinds.Phone.DISPLAY_NAME} ASC"
        )?.use { c ->
            while (c.moveToNext()) {
                val id = c.getLong(0)
                val number = c.getString(2) ?: continue
                val entry = byId.getOrPut(id) { C(c.getString(1) ?: "", c.getString(3), mutableListOf(), c.getLong(4)) }
                val plain = number.filter { it.isDigit() || it == '+' }
                if (entry.numbers.none { it.filter { ch -> ch.isDigit() || ch == '+' } == plain }) entry.numbers.add(number)
            }
        }
        val version = byId.entries.fold(17L) { h, (id, c) -> h * 31 + id * 7 + c.updated }.toString(16)
        val list = byId.map { (id, c) ->
            mapOf(
                "id" to id.toString(),
                "name" to c.name,
                "numbers" to c.numbers,
                "photo" to (c.photo?.let { photo(context, Uri.parse(it)) } ?: ""),
            )
        }
        return mapOf("version" to version, "contacts" to list)
    }

    private fun photo(context: Context, uri: Uri): String = try {
        context.contentResolver.openInputStream(uri)?.use { input ->
            val bmp = BitmapFactory.decodeStream(input) ?: return ""
            val out = ByteArrayOutputStream()
            Bitmap.createScaledBitmap(bmp, 64, 64, true).compress(Bitmap.CompressFormat.JPEG, 80, out)
            "data:image/jpeg;base64," + Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP)
        } ?: ""
    } catch (_: Exception) {
        ""
    }
}
