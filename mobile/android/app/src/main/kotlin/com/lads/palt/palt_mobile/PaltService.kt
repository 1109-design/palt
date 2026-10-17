package com.lads.palt.palt_mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.util.Log

/**
 * Foreground service that keeps the PALT process (and with it the Dart
 * engine) alive while the app's window is closed, like KDE Connect does.
 * It shows one quiet, permanent notification saying who is connected.
 */
class PaltService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        running = this
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "Connection", NotificationManager.IMPORTANCE_MIN).apply {
                description = "Shows that PALT is running and which devices are connected"
                setShowBadge(false)
            }
        )
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val n = buildNotification(status)
        try {
            if (Build.VERSION.SDK_INT >= 29) {
                startForeground(NOTIFICATION_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE)
            } else {
                startForeground(NOTIFICATION_ID, n)
            }
        } catch (e: Exception) {
            // Android refuses some background starts; the app still works while open.
            Log.w(TAG, "Could not start in the foreground: $e")
            stopSelf()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        running = null
        super.onDestroy()
    }

    private fun buildNotification(text: String): Notification {
        val open = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        return Notification.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle("PALT")
            .setContentText(text)
            .setContentIntent(open)
            .setOngoing(true)
            .setShowWhen(false)
            .setCategory(Notification.CATEGORY_SERVICE)
            .build()
    }

    companion object {
        private const val TAG = "PaltService"
        private const val CHANNEL_ID = "palt_service"
        private const val NOTIFICATION_ID = 1
        private var running: PaltService? = null
        private var status = "Looking for your devices"

        fun start(context: Context) {
            try {
                context.startForegroundService(Intent(context, PaltService::class.java))
            } catch (e: Exception) {
                Log.w(TAG, "Could not start service: $e")
            }
        }

        /** Updates the notification text, e.g. "Connected to laptop". */
        fun setStatus(context: Context, text: String) {
            status = text
            val s = running ?: return
            context.getSystemService(NotificationManager::class.java)
                .notify(NOTIFICATION_ID, s.buildNotification(text))
        }
    }
}
