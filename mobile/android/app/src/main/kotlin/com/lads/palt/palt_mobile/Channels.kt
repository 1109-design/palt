package com.lads.palt.palt_mobile

import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.net.ConnectivityManager
import android.net.Network
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.PowerManager
import android.provider.Settings
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.ByteArrayOutputStream
import java.util.concurrent.Executors

/** Platform channels of the engine, registered once when it is created. */
object Channels {
    private val worker = Executors.newSingleThreadExecutor()
    private val mainThread = Handler(Looper.getMainLooper())
    private var intentChannel: MethodChannel? = null

    /** Tells Dart there is a share or open-device request to take. */
    fun intentReady() {
        intentChannel?.invokeMethod("ready", null)
    }

    fun register(context: Context, engine: FlutterEngine) {
        val app = context.applicationContext
        val messenger = engine.dartExecutor.binaryMessenger

        // palt/media: video previews for the device thread (lib/utils/media.dart).
        MethodChannel(messenger, "palt/media").setMethodCallHandler { call, result ->
            when (call.method) {
                "videoThumbnail" -> {
                    val path = call.argument<String>("path")
                    val maxSide = call.argument<Int>("maxSide") ?: 480
                    if (path == null) {
                        result.error("args", "path is required", null)
                        return@setMethodCallHandler
                    }
                    worker.execute {
                        val out = try {
                            videoThumbnail(path, maxSide)
                        } catch (e: Exception) {
                            null
                        }
                        mainThread.post { result.success(out) }
                    }
                }
                else -> result.notImplemented()
            }
        }

        // palt/service: background service and battery settings
        // (lib/services/background.dart). Network changes are pushed to Dart.
        val service = MethodChannel(messenger, "palt/service")
        service.setMethodCallHandler { call, result ->
            when (call.method) {
                "start" -> {
                    PaltService.start(app)
                    result.success(null)
                }
                "setStatus" -> {
                    PaltService.setStatus(app, call.argument<String>("text") ?: "")
                    result.success(null)
                }
                "isIgnoringBatteryOptimizations" -> {
                    val pm = app.getSystemService(PowerManager::class.java)
                    result.success(pm.isIgnoringBatteryOptimizations(app.packageName))
                }
                "requestIgnoreBatteryOptimizations" -> {
                    val intent = Intent(
                        Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                        Uri.parse("package:${app.packageName}")
                    ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    try {
                        app.startActivity(intent)
                        result.success(true)
                    } catch (e: Exception) {
                        result.success(false)
                    }
                }
                else -> result.notImplemented()
            }
        }

        // palt/device: battery, signal, find my phone, notifications
        // (lib/features/). Battery changes and "Found it" are pushed to Dart.
        val device = MethodChannel(messenger, "palt/device")
        device.setMethodCallHandler { call, result ->
            when (call.method) {
                "battery" -> result.success(DeviceInfo.battery(app))
                "connectivity" -> result.success(DeviceInfo.connectivity(app))
                "ring" -> {
                    Ringer.start(app, call.argument<String>("from") ?: "Your computer")
                    result.success(null)
                }
                "stopRing" -> {
                    Ringer.stop(app)
                    result.success(null)
                }
                "notify" -> {
                    Notifier.show(
                        app,
                        call.argument<Int>("id") ?: 0,
                        call.argument<String>("title") ?: "PALT",
                        call.argument<String>("text") ?: "",
                        call.argument<String>("device"),
                        call.argument<String>("url"),
                    )
                    result.success(null)
                }
                "cancel" -> {
                    Notifier.cancel(app, call.argument<Int>("id") ?: 0)
                    result.success(null)
                }
                "wifiOffer" -> {
                    WifiShare.offer(
                        app,
                        call.argument<String>("ssid") ?: "",
                        call.argument<String>("password") ?: "",
                        call.argument<String>("security") ?: "",
                        call.argument<Boolean>("hidden") ?: false,
                        call.argument<String>("from") ?: "Your computer",
                    )
                    result.success(null)
                }
                "askTether" -> {
                    WifiShare.askTether(app, call.argument<String>("from") ?: "Your computer")
                    result.success(null)
                }
                "openTetherSettings" -> {
                    WifiShare.openTetherSettings(app)
                    result.success(null)
                }
                "openWifiSettings" -> {
                    WifiShare.openWifiSettings(app)
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
        DeviceInfo.watchBattery(app) { b -> mainThread.post { device.invokeMethod("battery", b) } }
        Ringer.onStopped = { mainThread.post { device.invokeMethod("ringStopped", null) } }

        // palt/notifications: the phone's notifications, mirrored to
        // computers (lib/features/notification_sync.dart).
        val notes = MethodChannel(messenger, "palt/notifications")
        notes.setMethodCallHandler { call, result ->
            val fwd = NotificationForwarder.instance
            when (call.method) {
                "hasAccess" -> result.success(hasNotificationAccess(app))
                "openAccessSettings" -> {
                    app.startActivity(
                        Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    )
                    result.success(null)
                }
                "dismiss" -> {
                    fwd?.dismiss(call.argument<String>("id") ?: "")
                    result.success(null)
                }
                "action" -> result.success(
                    fwd?.runAction(call.argument<String>("id") ?: "", call.argument<Int>("index") ?: -1) ?: false
                )
                "reply" -> result.success(
                    fwd?.reply(call.argument<String>("id") ?: "", call.argument<String>("text") ?: "") ?: false
                )
                else -> result.notImplemented()
            }
        }
        NotificationForwarder.sink = { kind, m -> mainThread.post { notes.invokeMethod(kind, m) } }

        // palt/clipboard: clipboard sync (lib/features/clipboard.dart).
        val clip = MethodChannel(messenger, "palt/clipboard")
        clip.setMethodCallHandler { call, result ->
            when (call.method) {
                "set" -> {
                    ClipboardBridge.set(app, call.argument<String>("text") ?: "")
                    result.success(null)
                }
                "get" -> result.success(ClipboardBridge.get(app))
                "autoAvailable" -> {
                    ClipboardWatcher.start(app)
                    result.success(
                        mapOf(
                            "logs" to ClipboardWatcher.available(app),
                            "onTop" to ClipboardWatcher.canAppearOnTop(app),
                        )
                    )
                }
                "openAppearOnTop" -> {
                    app.startActivity(
                        Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION, Uri.parse("package:${app.packageName}"))
                            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    )
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
        ClipboardBridge.onSend = { text -> mainThread.post { clip.invokeMethod("send", text) } }

        // palt/calls: incoming calls for computers (lib/features/calls.dart).
        val calls = MethodChannel(messenger, "palt/calls")
        calls.setMethodCallHandler { call, result ->
            when (call.method) {
                "mute" -> {
                    CallWatcher.mute(app)
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
        CallWatcher.sink = { m -> mainThread.post { calls.invokeMethod("call", m) } }
        CallWatcher.start(app)

        // palt/intent: shares and notification taps (lib/features/intake.dart).
        val intake = MethodChannel(messenger, "palt/intent")
        intake.setMethodCallHandler { call, result ->
            when (call.method) {
                "take" -> {
                    result.success(Intake.pending)
                    Intake.pending = null
                }
                else -> result.notImplemented()
            }
        }
        intentChannel = intake

        val cm = app.getSystemService(ConnectivityManager::class.java)
        cm.registerDefaultNetworkCallback(object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) = changed()
            override fun onLost(network: Network) = changed()
            // Internet access is confirmed a moment after a network comes up.
            private var validated: Boolean? = null
            override fun onCapabilitiesChanged(network: Network, caps: android.net.NetworkCapabilities) {
                val v = caps.hasCapability(android.net.NetworkCapabilities.NET_CAPABILITY_VALIDATED)
                if (v == validated) return
                validated = v
                mainThread.post { service.invokeMethod("statusChanged", null) }
            }
            private fun changed() {
                mainThread.post { service.invokeMethod("networkChanged", null) }
            }
        })
        // USB tethering on or off doesn't change the default network.
        val tether = object : android.content.BroadcastReceiver() {
            override fun onReceive(context: Context, intent: Intent) {
                mainThread.post { service.invokeMethod("networkChanged", null) }
            }
        }
        val tetherFilter = android.content.IntentFilter("android.net.conn.TETHER_STATE_CHANGED")
        if (Build.VERSION.SDK_INT >= 33) {
            app.registerReceiver(tether, tetherFilter, Context.RECEIVER_EXPORTED)
        } else {
            app.registerReceiver(tether, tetherFilter)
        }
    }

    private fun hasNotificationAccess(context: Context): Boolean {
        val enabled = Settings.Secure.getString(context.contentResolver, "enabled_notification_listeners") ?: ""
        return enabled.contains(context.packageName)
    }

    /** A JPEG frame (~1 s in) scaled to fit maxSide, plus the duration in ms. */
    private fun videoThumbnail(path: String, maxSide: Int): Map<String, Any>? {
        val retriever = MediaMetadataRetriever()
        try {
            retriever.setDataSource(path)
            val durationMs = retriever
                .extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)
                ?.toLongOrNull() ?: 0L
            // Skip the often-black first frame unless the clip is very short.
            val atUs = if (durationMs > 2000) 1_000_000L else 0L
            val frame: Bitmap = (
                if (Build.VERSION.SDK_INT >= 27) {
                    retriever.getScaledFrameAtTime(
                        atUs, MediaMetadataRetriever.OPTION_CLOSEST_SYNC, maxSide, maxSide
                    )
                } else {
                    retriever.getFrameAtTime(atUs, MediaMetadataRetriever.OPTION_CLOSEST_SYNC)
                }
            ) ?: return null
            val bytes = ByteArrayOutputStream()
            fitWithin(frame, maxSide).compress(Bitmap.CompressFormat.JPEG, 80, bytes)
            return mapOf("jpeg" to bytes.toByteArray(), "durationMs" to durationMs)
        } finally {
            try {
                retriever.release()
            } catch (_: Exception) {
            }
        }
    }

    private fun fitWithin(src: Bitmap, maxSide: Int): Bitmap {
        val longest = maxOf(src.width, src.height)
        if (longest <= maxSide) return src
        val scale = maxSide.toFloat() / longest
        return Bitmap.createScaledBitmap(
            src, (src.width * scale).toInt().coerceAtLeast(1),
            (src.height * scale).toInt().coerceAtLeast(1), true
        )
    }
}
