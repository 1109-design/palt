package com.lads.palt.palt_mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.Uri

/** Buttons on PALT's notifications. */
class ActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            STOP_RING -> Ringer.stop(context)
            OPEN_URL -> {
                val url = intent.getStringExtra(EXTRA_URL) ?: return
                Notifier.cancel(context, intent.getIntExtra(EXTRA_NOTIFICATION_ID, 0))
                try {
                    context.startActivity(
                        Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    )
                } catch (_: Exception) {
                }
            }
        }
    }

    companion object {
        const val STOP_RING = "com.lads.palt.STOP_RING"
        const val OPEN_URL = "com.lads.palt.OPEN_URL"
        const val EXTRA_URL = "url"
        const val EXTRA_NOTIFICATION_ID = "notificationId"
    }
}
