package com.lads.palt.palt_mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.drawable.Icon
import android.media.MediaMetadata
import android.media.session.MediaSession
import android.media.session.PlaybackState
import android.os.Handler
import android.os.Looper
import android.util.Base64
import java.net.URL

/**
 * What a computer is playing, in this phone's own media controls (quick
 * panel, lock screen, headset buttons), like GSConnect: a media session
 * that mirrors the computer's player, with a media notification. Presses
 * go to Dart (lib/features/media.dart), which sends them to the computer.
 *
 * MediaSessions skips this session so it isn't reported back.
 */
object RemoteMedia {
    private const val CHANNEL = "palt_remote_media"
    private const val NOTIFICATION_ID = 7341
    const val ACTION = "com.lads.palt.REMOTE_MEDIA"
    const val EXTRA_COMMAND = "command"

    /** Set by Channels: a control was pressed (action, value). */
    var onAction: ((String, Double) -> Unit)? = null

    private var session: MediaSession? = null
    private var artKey = ""
    private var artBitmap: Bitmap? = null
    private var last: Pair<Context, Map<String, Any?>>? = null
    private val main = Handler(Looper.getMainLooper())

    private fun session(context: Context): MediaSession =
        session ?: MediaSession(context, "PALT remote").also { s ->
            s.setCallback(object : MediaSession.Callback() {
                override fun onPlay() = send("play")
                override fun onPause() = send("pause")
                override fun onStop() = send("pause")
                override fun onSkipToNext() = send("next")
                override fun onSkipToPrevious() = send("previous")
                override fun onSeekTo(pos: Long) = send("seek", pos.toDouble())
            })
            s.setSessionActivity(Notifier.openAppIntent(context))
            session = s
        }

    fun send(action: String, value: Double = 0.0) {
        onAction?.invoke(action, value)
    }

    /** Shows or refreshes the computer's player. */
    fun update(context: Context, a: Map<String, Any?>) {
        val device = a["device"] as? String ?: ""
        val title = a["title"] as? String ?: ""
        val artist = a["artist"] as? String ?: ""
        val playing = a["playing"] as? Boolean ?: false
        val canNext = a["canNext"] as? Boolean ?: false
        val canPrev = a["canPrev"] as? Boolean ?: false
        val canSeek = a["canSeek"] as? Boolean ?: false
        val position = (a["position"] as? Number)?.toLong() ?: 0L
        val length = (a["length"] as? Number)?.toLong() ?: 0L
        last = context to a
        val art = art(a["art"] as? String ?: "")
        val s = session(context)

        s.setMetadata(
            MediaMetadata.Builder()
                .putString(MediaMetadata.METADATA_KEY_TITLE, title)
                .putString(MediaMetadata.METADATA_KEY_ARTIST, artist)
                .putString(MediaMetadata.METADATA_KEY_ALBUM, a["album"] as? String ?: "")
                .putString(MediaMetadata.METADATA_KEY_DISPLAY_SUBTITLE, "on $device")
                .putLong(MediaMetadata.METADATA_KEY_DURATION, length)
                .apply { if (art != null) putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, art) }
                .build()
        )
        var actions = PlaybackState.ACTION_PLAY or PlaybackState.ACTION_PAUSE or PlaybackState.ACTION_PLAY_PAUSE
        if (canNext) actions = actions or PlaybackState.ACTION_SKIP_TO_NEXT
        if (canPrev) actions = actions or PlaybackState.ACTION_SKIP_TO_PREVIOUS
        if (canSeek) actions = actions or PlaybackState.ACTION_SEEK_TO
        s.setPlaybackState(
            PlaybackState.Builder()
                .setActions(actions)
                .setState(
                    if (playing) PlaybackState.STATE_PLAYING else PlaybackState.STATE_PAUSED,
                    position, if (playing) 1f else 0f,
                )
                .build()
        )
        s.isActive = true

        ensureChannel(context)
        fun button(icon: Int, label: String, command: String, code: Int) = Notification.Action.Builder(
            Icon.createWithResource(context, icon), label,
            PendingIntent.getBroadcast(
                context, code,
                Intent(context, ActionReceiver::class.java).setAction(ACTION).putExtra(EXTRA_COMMAND, command),
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            ),
        ).build()

        val buttons = mutableListOf<Notification.Action>()
        if (canPrev) buttons += button(android.R.drawable.ic_media_previous, "Previous", "previous", 1)
        buttons += if (playing) button(android.R.drawable.ic_media_pause, "Pause", "pause", 2)
        else button(android.R.drawable.ic_media_play, "Play", "play", 2)
        if (canNext) buttons += button(android.R.drawable.ic_media_next, "Next", "next", 3)

        val n = Notification.Builder(context, CHANNEL)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(title)
            .setContentText(artist)
            .setSubText(device)
            .setLargeIcon(art)
            .setContentIntent(Notifier.openAppIntent(context, device))
            .setVisibility(Notification.VISIBILITY_PUBLIC)
            .setOnlyAlertOnce(true)
            .setOngoing(playing)
            .setStyle(
                Notification.MediaStyle()
                    .setMediaSession(s.sessionToken)
                    .setShowActionsInCompactView(*buttons.indices.toList().toIntArray())
            )
            .apply { buttons.forEach { addAction(it) } }
            .build()
        context.getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, n)
    }

    /** Nothing playing on any computer: take the player away. */
    fun clear(context: Context) {
        last = null
        context.getSystemService(NotificationManager::class.java).cancel(NOTIFICATION_ID)
        session?.let {
            it.isActive = false
            it.release()
        }
        session = null
    }

    /**
     * Art as sent by the computer, decoded once per track: a data: URL, or a
     * web URL fetched in the background (the player is refreshed when it lands).
     */
    private fun art(url: String): Bitmap? {
        if (url == artKey) return artBitmap
        artKey = url
        artBitmap = null
        if (url.startsWith("http://") || url.startsWith("https://")) {
            Thread {
                val bmp = try {
                    val conn = URL(url).openConnection().apply {
                        connectTimeout = 5000
                        readTimeout = 5000
                    }
                    conn.getInputStream().use { BitmapFactory.decodeStream(it) }
                } catch (_: Exception) {
                    null
                }
                main.post {
                    if (bmp != null && artKey == url) {
                        artBitmap = bmp
                        last?.let { (c, a) -> update(c, a) }
                    }
                }
            }.start()
            return null
        }
        artBitmap = try {
            val comma = url.indexOf(',')
            if (!url.startsWith("data:") || comma < 0) null
            else Base64.decode(url.substring(comma + 1), Base64.DEFAULT).let { BitmapFactory.decodeByteArray(it, 0, it.size) }
        } catch (_: Exception) {
            null
        }
        return artBitmap
    }

    private fun ensureChannel(context: Context) {
        context.getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, "Computer media", NotificationManager.IMPORTANCE_LOW).apply {
                description = "What your computer is playing, with controls"
                setShowBadge(false)
            }
        )
    }
}
