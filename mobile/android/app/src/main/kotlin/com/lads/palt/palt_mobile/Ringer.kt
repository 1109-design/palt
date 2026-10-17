package com.lads.palt.palt_mobile

import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.media.AudioAttributes
import android.media.AudioManager
import android.media.MediaPlayer
import android.media.RingtoneManager
import android.os.Handler
import android.os.Looper
import android.os.VibrationEffect
import android.os.Vibrator
import android.util.Log

/**
 * Find my phone: rings loudly on the alarm stream, which plays even in
 * silent mode and Do Not Disturb, until "Found it" or two minutes pass.
 */
object Ringer {
    private const val NOTIFICATION_ID = 7001
    private const val RING_MS = 120_000L

    private var player: MediaPlayer? = null
    private var savedVolume = -1
    private val main = Handler(Looper.getMainLooper())
    private val timeout = Runnable { stopNow() }
    private lateinit var app: Context

    /** Called when ringing stops, so Dart can tell the computer. */
    var onStopped: (() -> Unit)? = null

    fun start(context: Context, from: String) = main.post { startNow(context.applicationContext, from) }

    fun stop(context: Context) = main.post { stopNow() }

    val ringing get() = player != null

    private fun startNow(context: Context, from: String) {
        app = context
        if (player != null) return
        val audio = context.getSystemService(AudioManager::class.java)
        savedVolume = audio.getStreamVolume(AudioManager.STREAM_ALARM)
        audio.setStreamVolume(AudioManager.STREAM_ALARM, audio.getStreamMaxVolume(AudioManager.STREAM_ALARM), 0)

        val uri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_ALARM)
            ?: RingtoneManager.getDefaultUri(RingtoneManager.TYPE_RINGTONE)
        try {
            player = MediaPlayer().apply {
                setAudioAttributes(
                    AudioAttributes.Builder()
                        .setUsage(AudioAttributes.USAGE_ALARM)
                        .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                        .build()
                )
                setDataSource(context, uri)
                isLooping = true
                prepare()
                start()
            }
        } catch (e: Exception) {
            Log.w("Ringer", "Could not play alarm: $e")
        }
        context.getSystemService(Vibrator::class.java)?.vibrate(
            VibrationEffect.createWaveform(longArrayOf(0, 800, 400), 0)
        )

        Notifier.ensureChannels(context)
        val stop = PendingIntent.getBroadcast(
            context, NOTIFICATION_ID,
            Intent(context, ActionReceiver::class.java).setAction(ActionReceiver.STOP_RING),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val n = Notification.Builder(context, Notifier.CHANNEL_RING)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle("Ringing")
            .setContentText("$from is looking for this phone")
            .setOngoing(true)
            .setCategory(Notification.CATEGORY_ALARM)
            .setContentIntent(stop)
            .addAction(Notification.Action.Builder(null, "Found it", stop).build())
            .build()
        context.getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, n)
        main.postDelayed(timeout, RING_MS)
    }

    private fun stopNow() {
        main.removeCallbacks(timeout)
        val p = player ?: return
        player = null
        try {
            p.stop()
        } catch (_: Exception) {
        }
        p.release()
        app.getSystemService(Vibrator::class.java)?.cancel()
        if (savedVolume >= 0) {
            app.getSystemService(AudioManager::class.java)
                .setStreamVolume(AudioManager.STREAM_ALARM, savedVolume, 0)
        }
        Notifier.cancel(app, NOTIFICATION_ID)
        onStopped?.invoke()
    }
}
