package com.lads.palt.palt_mobile

import android.Manifest
import android.app.Activity
import android.app.PendingIntent
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.SystemClock
import android.util.Log
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

/**
 * Sends whatever is copied on the phone without a tap, once READ_LOGS has
 * been granted over adb (`pm grant <pkg> android.permission.READ_LOGS`).
 *
 * Android only lets the app on screen read the clipboard, but it still
 * calls every app's clipboard listener on a copy, and logs that it denied
 * the read: "Denying clipboard access to <pkg>". PALT keeps a listener
 * registered, watches the log for its own name, and then opens
 * [ClipboardActivity] for a split second to read the new clip.
 *
 * Since Android 13 a log reader sees other apps' lines only after the user
 * allows it in a prompt that appears only while PALT is on screen, so
 * [start] is called from MainActivity and the reader is kept running.
 */
object ClipboardWatcher {
    private const val TAG = "PaltClipboard"
    private var thread: Thread? = null
    private var process: Process? = null
    private val listener = ClipboardManager.OnPrimaryClipChangedListener { }

    fun available(context: Context) =
        context.checkSelfPermission(Manifest.permission.READ_LOGS) == PackageManager.PERMISSION_GRANTED

    @Synchronized
    fun start(context: Context) {
        if (thread != null || !available(context)) return
        val app = context.applicationContext
        app.getSystemService(ClipboardManager::class.java).addPrimaryClipChangedListener(listener)
        val needle = "clipboard access to ${app.packageName}"
        thread = Thread {
            try {
                // -T 1: start from the newest line rather than the whole buffer.
                val p = ProcessBuilder("logcat", "-T", "1", "-v", "brief", "-s", "ClipboardService:E")
                    .redirectErrorStream(true).start()
                process = p
                val started = SystemClock.elapsedRealtime()
                var lastOpen = 0L
                p.inputStream.bufferedReader().forEachLine { line ->
                    val now = SystemClock.elapsedRealtime()
                    // Skip the one old line -T replays, and repeats of a copy.
                    if (!line.contains(needle) || now - started < 500 || now - lastOpen < 700) return@forEachLine
                    lastOpen = now
                    Log.d(TAG, "Copy seen, reading the clipboard")
                    try {
                        app.startActivity(
                            Intent(app, ClipboardActivity::class.java)
                                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_NO_ANIMATION)
                        )
                    } catch (e: Exception) {
                        Log.w(TAG, "Couldn't open the clipboard reader: $e")
                    }
                }
            } catch (e: Exception) {
                Log.w(TAG, "Clipboard log watch stopped: $e")
            } finally {
                synchronized(this) { thread = null; process = null }
            }
        }.apply { isDaemon = true; name = "palt-clipboard-log"; start() }
        Log.i(TAG, "Watching for copies")

    }

    @Synchronized
    fun stop(context: Context) {
        context.applicationContext.getSystemService(ClipboardManager::class.java)
            .removePrimaryClipChangedListener(listener)
        process?.destroy()
    }
}

/** Invisible; reads the clipboard once it has focus, hands it over, closes. */
class ClipboardActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        overridePendingTransition(0, 0)
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
