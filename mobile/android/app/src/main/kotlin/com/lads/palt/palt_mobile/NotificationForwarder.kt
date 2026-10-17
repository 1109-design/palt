package com.lads.palt.palt_mobile

import android.app.Notification
import android.app.RemoteInput
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.drawable.Drawable
import android.os.Bundle
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import android.util.Base64
import android.util.Log
import java.io.ByteArrayOutputStream

/**
 * Mirrors the phone's notifications to paired computers (GSConnect's
 * notification sync). Needs "Notification access", granted by the user in
 * Android settings. Posted/removed notifications go to Dart, which filters
 * and forwards them; dismiss, actions and replies come back from Dart.
 */
class NotificationForwarder : NotificationListenerService() {

    override fun onListenerConnected() {
        instance = this
    }

    override fun onListenerDisconnected() {
        if (instance === this) instance = null
    }

    override fun onNotificationPosted(sbn: StatusBarNotification) {
        if (!worthForwarding(sbn)) return
        val m = try {
            describe(sbn)
        } catch (e: Exception) {
            Log.w(TAG, "Could not read notification: $e")
            return
        }
        sink?.invoke("posted", m)
    }

    override fun onNotificationRemoved(sbn: StatusBarNotification) {
        if (sbn.packageName == packageName) return
        sink?.invoke("removed", mapOf("id" to sbn.key))
    }

    private fun worthForwarding(sbn: StatusBarNotification): Boolean {
        val n = sbn.notification
        if (sbn.packageName == packageName) return false
        if (sbn.isOngoing || !sbn.isClearable) return false
        if (n.flags and Notification.FLAG_GROUP_SUMMARY != 0) return false
        if (sbn.packageName == "android" || sbn.packageName == "com.android.systemui") return false
        val e = n.extras
        return !e.getCharSequence(Notification.EXTRA_TITLE).isNullOrBlank() ||
            !e.getCharSequence(Notification.EXTRA_TEXT).isNullOrBlank()
    }

    private fun describe(sbn: StatusBarNotification): Map<String, Any?> {
        val n = sbn.notification
        val e = n.extras
        val title = e.getCharSequence(Notification.EXTRA_TITLE)?.toString() ?: ""
        val text = (e.getCharSequence(Notification.EXTRA_BIG_TEXT) ?: e.getCharSequence(Notification.EXTRA_TEXT))
            ?.toString() ?: ""
        val actions = n.actions ?: emptyArray()
        val labels = ArrayList<String>()
        var replyable = false
        for (a in actions) {
            if (a.remoteInputs?.isNotEmpty() == true) replyable = true else labels.add(a.title?.toString() ?: "")
        }
        return mapOf(
            "id" to sbn.key,
            "appId" to sbn.packageName,
            "app" to appName(sbn.packageName),
            "title" to title,
            "text" to text,
            "time" to sbn.postTime,
            "actions" to labels,
            "replyable" to replyable,
            "silent" to (n.flags and Notification.FLAG_ONLY_ALERT_ONCE != 0 && sbn.postTime != n.`when`),
            "icon" to appIcon(sbn.packageName),
        )
    }

    private fun appName(pkg: String): String = try {
        val info = packageManager.getApplicationInfo(pkg, 0)
        packageManager.getApplicationLabel(info).toString()
    } catch (_: Exception) {
        pkg
    }

    /** The app's icon as a small PNG, base64, cached per app. */
    private fun appIcon(pkg: String): String? {
        icons[pkg]?.let { return it }
        return try {
            val png = drawableToPng(packageManager.getApplicationIcon(pkg), 96)
            Base64.encodeToString(png, Base64.NO_WRAP).also { icons[pkg] = it }
        } catch (_: Exception) {
            null
        }
    }

    private fun drawableToPng(d: Drawable, size: Int): ByteArray {
        val bmp = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
        d.setBounds(0, 0, size, size)
        d.draw(Canvas(bmp))
        val out = ByteArrayOutputStream()
        bmp.compress(Bitmap.CompressFormat.PNG, 100, out)
        return out.toByteArray()
    }

    private fun find(key: String): StatusBarNotification? =
        try {
            activeNotifications?.firstOrNull { it.key == key }
        } catch (_: Exception) {
            null
        }

    fun dismiss(key: String) {
        try {
            cancelNotification(key)
        } catch (_: Exception) {
        }
    }

    /** Runs the [index]th non-reply action. */
    fun runAction(key: String, index: Int): Boolean {
        val actions = find(key)?.notification?.actions ?: return false
        val plain = actions.filter { it.remoteInputs.isNullOrEmpty() }
        val a = plain.getOrNull(index) ?: return false
        return try {
            a.actionIntent.send()
            true
        } catch (_: Exception) {
            false
        }
    }

    /** Answers through the notification's reply action, like typing in it. */
    fun reply(key: String, text: String): Boolean {
        val actions = find(key)?.notification?.actions ?: return false
        val a = actions.firstOrNull { !it.remoteInputs.isNullOrEmpty() } ?: return false
        val inputs = a.remoteInputs
        val fill = Intent()
        val results = Bundle()
        for (ri in inputs) results.putCharSequence(ri.resultKey, text)
        RemoteInput.addResultsToIntent(inputs, fill, results)
        return try {
            a.actionIntent.send(this, 0, fill)
            true
        } catch (_: Exception) {
            false
        }
    }

    companion object {
        private const val TAG = "PaltNotifications"
        private val icons = HashMap<String, String>()

        @Volatile
        var instance: NotificationForwarder? = null

        /** Set by Channels: ("posted" | "removed", details). */
        var sink: ((String, Map<String, Any?>) -> Unit)? = null
    }
}
