package com.lads.palt.palt_mobile

import android.content.ComponentName
import android.content.Context
import android.graphics.Bitmap
import android.media.AudioManager
import android.media.MediaMetadata
import android.media.session.MediaController
import android.media.session.MediaSessionManager
import android.media.session.PlaybackState
import android.os.Handler
import android.os.Looper
import android.util.Base64
import java.io.ByteArrayOutputStream

/**
 * What is playing on this phone, for computers' media remote
 * (lib/features/media.dart): the media sessions of every app, read and
 * controlled through MediaSessionManager, which the notification access
 * PALT already has unlocks.
 */
object MediaSessions {
    /** Set by Channels: called (batched) whenever something changes. */
    var onChange: (() -> Unit)? = null

    private val main = Handler(Looper.getMainLooper())
    private var manager: MediaSessionManager? = null
    private val callbacks = mutableMapOf<MediaController, MediaController.Callback>()
    private var artCache: Pair<Any?, String> = null to ""

    private val changed = Runnable { onChange?.invoke() }

    private fun poke() {
        main.removeCallbacks(changed)
        main.postDelayed(changed, 300)
    }

    private fun listener(context: Context) = ComponentName(context, NotificationForwarder::class.java)

    /** Starts following sessions; false without notification access. */
    fun start(context: Context): Boolean {
        if (manager != null) return true
        val m = context.getSystemService(MediaSessionManager::class.java)
        return try {
            m.addOnActiveSessionsChangedListener({ watch(it.orEmpty()) }, listener(context), main)
            manager = m
            watch(m.getActiveSessions(listener(context)))
            val audio = context.getSystemService(AudioManager::class.java)
            // Volume keys and sliders: follow music volume changes too.
            context.contentResolver.registerContentObserver(
                android.provider.Settings.System.CONTENT_URI, true,
                object : android.database.ContentObserver(main) {
                    private var last = audio.getStreamVolume(AudioManager.STREAM_MUSIC)
                    override fun onChange(selfChange: Boolean) {
                        val v = audio.getStreamVolume(AudioManager.STREAM_MUSIC)
                        if (v != last) {
                            last = v
                            poke()
                        }
                    }
                }
            )
            true
        } catch (_: SecurityException) {
            false
        }
    }

    private fun watch(controllers: List<MediaController>) {
        for ((c, cb) in callbacks) c.unregisterCallback(cb)
        callbacks.clear()
        for (c in controllers) {
            val cb = object : MediaController.Callback() {
                override fun onPlaybackStateChanged(state: PlaybackState?) = poke()
                override fun onMetadataChanged(metadata: MediaMetadata?) = poke()
                override fun onSessionDestroyed() = poke()
            }
            c.registerCallback(cb, main)
            callbacks[c] = cb
        }
        poke()
    }

    // Skips PALT's own session (RemoteMedia): it shows a computer's player.
    private fun controllers(context: Context): List<MediaController> = try {
        (manager ?: context.getSystemService(MediaSessionManager::class.java))
            .getActiveSessions(listener(context))
            .filter { it.packageName != context.packageName }
    } catch (_: SecurityException) {
        emptyList()
    }

    /** The phone's media state, shaped like the computer's MediaState. */
    fun state(context: Context): Map<String, Any> {
        val pm = context.packageManager
        val players = controllers(context).mapNotNull { c ->
            val meta = c.metadata ?: return@mapNotNull null
            val ps = c.playbackState
            val actions = ps?.actions ?: 0L
            val name = try {
                pm.getApplicationLabel(pm.getApplicationInfo(c.packageName, 0)).toString()
            } catch (_: Exception) {
                c.packageName
            }
            var position = ps?.position ?: 0L
            if (ps?.state == PlaybackState.STATE_PLAYING && ps.lastPositionUpdateTime > 0) {
                position += ((android.os.SystemClock.elapsedRealtime() - ps.lastPositionUpdateTime) * ps.playbackSpeed).toLong()
            }
            mapOf(
                "id" to c.packageName,
                "name" to name,
                "title" to (meta.getString(MediaMetadata.METADATA_KEY_TITLE) ?: ""),
                "artist" to (meta.getString(MediaMetadata.METADATA_KEY_ARTIST)
                    ?: meta.getString(MediaMetadata.METADATA_KEY_ALBUM_ARTIST) ?: ""),
                "album" to (meta.getString(MediaMetadata.METADATA_KEY_ALBUM) ?: ""),
                "art" to art(meta),
                "playing" to (ps?.state == PlaybackState.STATE_PLAYING),
                "canNext" to (actions and PlaybackState.ACTION_SKIP_TO_NEXT != 0L),
                "canPrev" to (actions and PlaybackState.ACTION_SKIP_TO_PREVIOUS != 0L),
                "canSeek" to (actions and PlaybackState.ACTION_SEEK_TO != 0L),
                "canControl" to true,
                "position" to position.coerceAtLeast(0),
                "length" to meta.getLong(MediaMetadata.METADATA_KEY_DURATION).coerceAtLeast(0),
                "volume" to -1.0,
            )
        }
        val audio = context.getSystemService(AudioManager::class.java)
        val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC).coerceAtLeast(1)
        return mapOf(
            "players" to players,
            "volume" to mapOf(
                "level" to audio.getStreamVolume(AudioManager.STREAM_MUSIC) * 100 / max,
                "muted" to audio.isStreamMute(AudioManager.STREAM_MUSIC),
            ),
            "at" to System.currentTimeMillis(),
        )
    }

    /** Cover art as a small JPEG data URL, cached for the current track. */
    private fun art(meta: MediaMetadata): String {
        val bmp = meta.getBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART)
            ?: meta.getBitmap(MediaMetadata.METADATA_KEY_ART)
            ?: return meta.getString(MediaMetadata.METADATA_KEY_ALBUM_ART_URI)
                ?.takeIf { it.startsWith("http") } ?: ""
        if (artCache.first === bmp) return artCache.second
        val side = 256
        val scale = minOf(1f, side.toFloat() / maxOf(bmp.width, bmp.height))
        val small = Bitmap.createScaledBitmap(
            bmp, (bmp.width * scale).toInt().coerceAtLeast(1), (bmp.height * scale).toInt().coerceAtLeast(1), true
        )
        val out = ByteArrayOutputStream()
        small.compress(Bitmap.CompressFormat.JPEG, 80, out)
        val url = "data:image/jpeg;base64," + Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP)
        artCache = bmp to url
        return url
    }

    /** Runs a remote control action; player "" means the music volume. */
    fun action(context: Context, player: String, action: String, value: Double) {
        if (player.isEmpty()) {
            val audio = context.getSystemService(AudioManager::class.java)
            when (action) {
                "volume" -> {
                    val max = audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
                    audio.setStreamVolume(AudioManager.STREAM_MUSIC, (value.coerceIn(0.0, 100.0) * max / 100).toInt(), 0)
                }
                "mute" -> audio.adjustStreamVolume(AudioManager.STREAM_MUSIC, AudioManager.ADJUST_TOGGLE_MUTE, 0)
            }
            poke()
            return
        }
        val c = controllers(context).firstOrNull { it.packageName == player } ?: return
        val t = c.transportControls
        when (action) {
            "playpause" -> if (c.playbackState?.state == PlaybackState.STATE_PLAYING) t.pause() else t.play()
            "play" -> t.play()
            "pause" -> t.pause()
            "next" -> t.skipToNext()
            "previous" -> t.skipToPrevious()
            "seek" -> t.seekTo(value.toLong())
        }
    }
}
