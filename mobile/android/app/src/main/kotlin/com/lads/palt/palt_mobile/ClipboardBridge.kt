package com.lads.palt.palt_mobile

import android.app.Activity
import android.app.PendingIntent
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.service.quicksettings.TileService

/**
 * Clipboard sync. Writing works from the background; reading only works for
 * the app on screen, so "Send clipboard" (tile, notification button) opens
 * [ClipboardActivity] for a split second, which reads it and closes.
 */
object ClipboardBridge {
    /** Set by Channels: called with clipboard text to send. */
    var onSend: ((String) -> Unit)? = null

    fun set(context: Context, text: String) {
        val cm = context.getSystemService(ClipboardManager::class.java)
        cm.setPrimaryClip(ClipData.newPlainText("PALT", text))
    }

    fun get(context: Context): String? {
        val cm = context.getSystemService(ClipboardManager::class.java)
        val clip = cm.primaryClip ?: return null
        if (clip.itemCount == 0) return null
        return clip.getItemAt(0).coerceToText(context)?.toString()
    }

    fun sendIntent(context: Context): PendingIntent = PendingIntent.getActivity(
        context, 42,
        Intent(context, ClipboardActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
    )
}

/** Invisible; reads the clipboard once it has focus, hands it over, closes. */
class ClipboardActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (!hasFocus) return
        ClipboardBridge.get(this)?.takeIf { it.isNotBlank() }?.let { ClipboardBridge.onSend?.invoke(it) }
        finish()
    }
}

/** Quick Settings tile: "Send clipboard" to paired computers. */
class ClipboardTile : TileService() {
    override fun onClick() {
        super.onClick()
        if (Build.VERSION.SDK_INT >= 34) {
            startActivityAndCollapse(ClipboardBridge.sendIntent(this))
        } else {
            @Suppress("DEPRECATION")
            startActivityAndCollapse(Intent(this, ClipboardActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        }
    }
}
