package com.lads.palt.palt_mobile

import android.Manifest
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothClass
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings

/**
 * This phone's Bluetooth for computers (lib/features/bluetooth.dart): on or
 * off, connected devices with battery, and turning it on or off. Android 13+
 * doesn't let apps switch Bluetooth: "on" shows the system prompt, "off"
 * opens Bluetooth settings.
 */
object BluetoothInfo {
    /** Set by Channels: Bluetooth changed. */
    var onChange: (() -> Unit)? = null
    private var receiver: BroadcastReceiver? = null
    // Hidden broadcast for headset battery changes.
    private const val BATTERY_CHANGED = "android.bluetooth.device.action.BATTERY_LEVEL_CHANGED"

    private fun canConnect(context: Context) = Build.VERSION.SDK_INT < Build.VERSION_CODES.S ||
        context.checkSelfPermission(Manifest.permission.BLUETOOTH_CONNECT) == PackageManager.PERMISSION_GRANTED

    private fun adapter(context: Context): BluetoothAdapter? =
        context.getSystemService(BluetoothManager::class.java)?.adapter

    fun watch(context: Context) {
        if (receiver != null) return
        val r = object : BroadcastReceiver() {
            override fun onReceive(c: Context, i: Intent) {
                onChange?.invoke()
            }
        }
        val filter = IntentFilter().apply {
            addAction(BluetoothAdapter.ACTION_STATE_CHANGED)
            addAction(BluetoothDevice.ACTION_ACL_CONNECTED)
            addAction(BluetoothDevice.ACTION_ACL_DISCONNECTED)
            addAction(BATTERY_CHANGED)
        }
        if (Build.VERSION.SDK_INT >= 33) {
            context.applicationContext.registerReceiver(r, filter, Context.RECEIVER_EXPORTED)
        } else {
            context.applicationContext.registerReceiver(r, filter)
        }
        receiver = r
    }

    @Suppress("MissingPermission")
    fun state(context: Context): Map<String, Any> {
        val a = adapter(context) ?: return mapOf("available" to false, "on" to false, "devices" to emptyList<Any>())
        val on = try {
            a.isEnabled
        } catch (_: SecurityException) {
            false
        }
        val devices = mutableListOf<Map<String, Any>>()
        if (on && canConnect(context)) {
            try {
                for (d in a.bondedDevices ?: emptySet()) {
                    if (!connected(d)) continue
                    devices.add(
                        mapOf(
                            "name" to ((if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) d.alias else null) ?: d.name ?: d.address),
                            "kind" to kind(d),
                            "battery" to battery(d),
                        )
                    )
                }
            } catch (_: SecurityException) {
            }
        }
        return mapOf("available" to true, "on" to on, "devices" to devices)
    }

    // isConnected and getBatteryLevel are hidden but allowed APIs.
    private fun connected(d: BluetoothDevice): Boolean = try {
        d.javaClass.getMethod("isConnected").invoke(d) as Boolean
    } catch (_: Exception) {
        false
    }

    private fun battery(d: BluetoothDevice): Int = try {
        (d.javaClass.getMethod("getBatteryLevel").invoke(d) as Int).takeIf { it in 0..100 } ?: -1
    } catch (_: Exception) {
        -1
    }

    @Suppress("MissingPermission")
    private fun kind(d: BluetoothDevice): String = when (d.bluetoothClass?.majorDeviceClass) {
        BluetoothClass.Device.Major.AUDIO_VIDEO -> "headset"
        BluetoothClass.Device.Major.PERIPHERAL -> "input"
        BluetoothClass.Device.Major.COMPUTER -> "computer"
        BluetoothClass.Device.Major.PHONE -> "phone"
        BluetoothClass.Device.Major.WEARABLE -> "watch"
        else -> ""
    }

    /** Turns Bluetooth on or off as far as Android allows: "done", "asked" or "settings". */
    @Suppress("DEPRECATION", "MissingPermission")
    fun set(context: Context, on: Boolean): String {
        val a = adapter(context) ?: throw IllegalStateException("This phone has no Bluetooth")
        if (Build.VERSION.SDK_INT < 33) {
            if (if (on) a.enable() else a.disable()) return "done"
        }
        if (on) {
            if (!canConnect(context)) throw SecurityException("Allow PALT to use Bluetooth on the phone first")
            context.startActivity(Intent(BluetoothAdapter.ACTION_REQUEST_ENABLE).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            return "asked"
        }
        context.startActivity(Intent(Settings.ACTION_BLUETOOTH_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        return "settings"
    }
}
