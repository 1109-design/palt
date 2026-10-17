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

        val cm = app.getSystemService(ConnectivityManager::class.java)
        cm.registerDefaultNetworkCallback(object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) = changed()
            override fun onLost(network: Network) = changed()
            private fun changed() {
                mainThread.post { service.invokeMethod("networkChanged", null) }
            }
        })
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
