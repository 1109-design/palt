package com.lads.palt.palt_mobile

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.BatteryManager
import android.os.Build
import android.telephony.TelephonyManager

/** Battery and mobile signal, reported to paired computers. */
object DeviceInfo {
    fun battery(context: Context): Map<String, Any> {
        val i = context.registerReceiver(null, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
        return batteryFrom(i)
    }

    fun batteryFrom(i: Intent?): Map<String, Any> {
        val level = i?.getIntExtra(BatteryManager.EXTRA_LEVEL, -1) ?: -1
        val scale = i?.getIntExtra(BatteryManager.EXTRA_SCALE, 100) ?: 100
        val status = i?.getIntExtra(BatteryManager.EXTRA_STATUS, -1) ?: -1
        val pct = if (level >= 0 && scale > 0) level * 100 / scale else -1
        val charging = status == BatteryManager.BATTERY_STATUS_CHARGING ||
            status == BatteryManager.BATTERY_STATUS_FULL
        return mapOf("level" to pct, "charging" to charging)
    }

    /** Calls [onChange] whenever level or charging changes. */
    fun watchBattery(context: Context, onChange: (Map<String, Any>) -> Unit) {
        var last: Map<String, Any>? = null
        context.registerReceiver(object : BroadcastReceiver() {
            override fun onReceive(c: Context, i: Intent) {
                val b = batteryFrom(i)
                if (b != last) {
                    last = b
                    onChange(b)
                }
            }
        }, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
    }

    fun connectivity(context: Context): Map<String, Any> {
        val tm = context.getSystemService(TelephonyManager::class.java)
        val signal = if (Build.VERSION.SDK_INT >= 28) tm.signalStrength?.level ?: -1 else -1
        var network = ""
        if (context.checkSelfPermission(Manifest.permission.READ_PHONE_STATE) == PackageManager.PERMISSION_GRANTED) {
            network = try {
                networkName(tm.dataNetworkType)
            } catch (_: SecurityException) {
                ""
            }
        }
        return mapOf("network" to network, "signal" to signal)
    }

    private fun networkName(t: Int): String = when (t) {
        TelephonyManager.NETWORK_TYPE_NR -> "5G"
        TelephonyManager.NETWORK_TYPE_LTE -> "LTE"
        TelephonyManager.NETWORK_TYPE_HSPAP, TelephonyManager.NETWORK_TYPE_HSPA,
        TelephonyManager.NETWORK_TYPE_HSDPA, TelephonyManager.NETWORK_TYPE_HSUPA -> "3G+"
        TelephonyManager.NETWORK_TYPE_UMTS, TelephonyManager.NETWORK_TYPE_EVDO_0,
        TelephonyManager.NETWORK_TYPE_EVDO_A, TelephonyManager.NETWORK_TYPE_EVDO_B -> "3G"
        TelephonyManager.NETWORK_TYPE_EDGE -> "EDGE"
        TelephonyManager.NETWORK_TYPE_GPRS -> "GPRS"
        TelephonyManager.NETWORK_TYPE_UNKNOWN -> ""
        else -> "2G"
    }
}
