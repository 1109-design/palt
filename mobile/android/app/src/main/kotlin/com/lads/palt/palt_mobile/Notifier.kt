package com.lads.palt.palt_mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent

/** PALT's own notifications: incoming messages, pings, links. */
object Notifier {
    const val CHANNEL_MESSAGES = "palt_messages"
    const val CHANNEL_RING = "palt_ring"

    fun ensureChannels(context: Context) {
        val nm = context.getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_MESSAGES, "Messages", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "Messages, links and pings from your devices"
            }
        )
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_RING, "Find my phone", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "Shown while your computer is ringing this phone"
                setSound(null, null)
            }
        )
    }

    fun openAppIntent(context: Context, device: String? = null): PendingIntent {
        val intent = Intent(context, MainActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_NEW_TASK)
        if (device != null) intent.putExtra(MainActivity.EXTRA_OPEN_DEVICE, device)
        return PendingIntent.getActivity(
            context, device?.hashCode() ?: 0, intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
    }

    /** Shows a message; [url] adds an "Open link" button. */
    fun show(context: Context, id: Int, title: String, text: String, device: String?, url: String?) {
        ensureChannels(context)
        val b = Notification.Builder(context, CHANNEL_MESSAGES)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(title)
            .setContentText(text)
            .setStyle(Notification.BigTextStyle().bigText(text))
            .setContentIntent(openAppIntent(context, device))
            .setAutoCancel(true)
            .setCategory(Notification.CATEGORY_MESSAGE)
        if (url != null) {
            val open = Intent(context, ActionReceiver::class.java)
                .setAction(ActionReceiver.OPEN_URL)
                .putExtra(ActionReceiver.EXTRA_URL, url)
                .putExtra(ActionReceiver.EXTRA_NOTIFICATION_ID, id)
            b.addAction(
                Notification.Action.Builder(
                    null, "Open link",
                    PendingIntent.getBroadcast(
                        context, id, open,
                        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
                    )
                ).build()
            )
        }
        context.getSystemService(NotificationManager::class.java).notify(id, b.build())
    }

    fun cancel(context: Context, id: Int) {
        context.getSystemService(NotificationManager::class.java).cancel(id)
    }
}
