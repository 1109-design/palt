package com.lads.palt.palt_mobile

import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.os.Build
import android.os.Handler
import android.os.Looper
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import java.io.ByteArrayOutputStream
import java.util.concurrent.Executors

class MainActivity : FlutterActivity() {
    private val worker = Executors.newSingleThreadExecutor()
    private val mainThread = Handler(Looper.getMainLooper())

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        // palt/media: video previews for the device thread (lib/utils/media.dart).
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "palt/media")
            .setMethodCallHandler { call, result ->
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
