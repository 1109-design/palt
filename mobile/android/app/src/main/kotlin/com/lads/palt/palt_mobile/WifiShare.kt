package com.lads.palt.palt_mobile

import android.app.Activity
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.wifi.WifiNetworkSuggestion
import android.os.Build
import android.os.Bundle
import android.provider.Settings

/**
 * Wi-Fi networks shared by a computer (lib/features/wifi_share.dart): a
 * "Join X?" notification whose button opens Android's own "Save this
 * network?" sheet, so the user always confirms.
 */
object WifiShare {
    const val CHANNEL = "palt_wifi"
    private const val NOTIFICATION_ID = 7300

    fun offer(context: Context, ssid: String, password: String, security: String, hidden: Boolean, from: String) {
        val nm = context.getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL, "Shared Wi-Fi", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "Wi-Fi networks your computer shares with this phone"
            }
        )
        val join = Intent(context, WifiAddActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            .putExtra(WifiAddActivity.EXTRA_SSID, ssid)
            .putExtra(WifiAddActivity.EXTRA_PASSWORD, password)
            .putExtra(WifiAddActivity.EXTRA_SECURITY, security)
            .putExtra(WifiAddActivity.EXTRA_HIDDEN, hidden)
            .putExtra(WifiAddActivity.EXTRA_NOTIFICATION_ID, NOTIFICATION_ID)
        val pi = PendingIntent.getActivity(
            context, NOTIFICATION_ID, join,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        val n = Notification.Builder(context, CHANNEL)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle("Join $ssid?")
            .setContentText("$from shared this Wi-Fi network")
            .setContentIntent(pi)
            .addAction(Notification.Action.Builder(null, "Join", pi).build())
            .setAutoCancel(true)
            .build()
        nm.notify(NOTIFICATION_ID, n)
    }

    /**
     * Opens the hotspot and tethering settings (no public intent: try the
     * settings app's own screen, else the network settings).
     */
    fun openTetherSettings(context: Context) {
        val tether = Intent().setClassName("com.android.settings", "com.android.settings.TetherSettings")
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        try {
            context.startActivity(tether)
        } catch (_: Exception) {
            context.startActivity(Intent(Settings.ACTION_WIRELESS_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        }
    }

    /** Opens the Wi-Fi settings, where the phone's network can be shared as a QR code. */
    fun openWifiSettings(context: Context) {
        context.startActivity(Intent(Settings.ACTION_WIFI_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }
}

/** Shows Android's "Save this network?" sheet for a shared network, then closes. */
class WifiAddActivity : Activity() {
    companion object {
        const val EXTRA_SSID = "ssid"
        const val EXTRA_PASSWORD = "password"
        const val EXTRA_SECURITY = "security"
        const val EXTRA_HIDDEN = "hidden"
        const val EXTRA_NOTIFICATION_ID = "notification_id"
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        getSystemService(NotificationManager::class.java)
            .cancel(intent.getIntExtra(EXTRA_NOTIFICATION_ID, 0))
        val ssid = intent.getStringExtra(EXTRA_SSID).orEmpty()
        val password = intent.getStringExtra(EXTRA_PASSWORD).orEmpty()
        val security = intent.getStringExtra(EXTRA_SECURITY).orEmpty()
        // The add-networks sheet needs Android 11; WEP can't be suggested.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R || ssid.isEmpty() || security == "WEP") {
            WifiShare.openWifiSettings(this)
            finish()
            return
        }
        val b = WifiNetworkSuggestion.Builder().setSsid(ssid)
        if (security == "WPA" && password.isNotEmpty()) b.setWpa2Passphrase(password)
        if (intent.getBooleanExtra(EXTRA_HIDDEN, false)) b.setIsHiddenSsid(true)
        val add = Intent(Settings.ACTION_WIFI_ADD_NETWORKS)
            .putParcelableArrayListExtra(Settings.EXTRA_WIFI_NETWORK_LIST, arrayListOf(b.build()))
        try {
            startActivityForResult(add, 1)
        } catch (_: Exception) {
            WifiShare.openWifiSettings(this)
            finish()
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        finish()
    }
}
