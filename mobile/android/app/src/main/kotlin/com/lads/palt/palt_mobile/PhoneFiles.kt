package com.lads.palt.palt_mobile

import android.Manifest
import android.annotation.TargetApi
import android.app.ActivityManager
import android.app.usage.StorageStatsManager
import android.app.Activity
import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.database.ContentObserver
import android.graphics.Bitmap
import android.media.MediaScannerConnection
import android.media.ThumbnailUtils
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.os.Handler
import android.os.Looper
import android.provider.MediaStore
import android.os.StatFs
import android.os.storage.StorageManager
import android.provider.Settings
import android.util.Size
import java.io.ByteArrayOutputStream
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Phone storage for computers' file browser (lib/features/phone_files.dart):
 * "All files access", thumbnails, media rescans, photos taken on request,
 * and spotting Wi-Fi QR screenshots to offer them to a computer. Needs
 * Android 11 (Channels checks).
 */
@TargetApi(Build.VERSION_CODES.R)
object PhoneFiles {
    /** Set by Channels: a requested photo was saved (path) or cancelled (null). */
    var onPhoto: ((String?) -> Unit)? = null
    /** Set by Channels: a new screenshot appeared. */
    var onScreenshot: ((String) -> Unit)? = null
    /** Set by Channels: "Send to <computer>" was tapped on a Wi-Fi QR screenshot. */
    var onSendWifi: ((String, String) -> Unit)? = null

    const val SEND_WIFI = "com.lads.palt.SEND_WIFI"
    const val EXTRA_PATH = "path"
    const val EXTRA_COMPUTER = "computer"
    private const val WIFI_NOTIFICATION = 7342

    fun hasAccess(): Boolean = Environment.isExternalStorageManager()

    fun openAccessSettings(context: Context) {
        context.startActivity(
            Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, Uri.parse("package:${context.packageName}"))
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        )
    }

    /** A JPEG preview of a picture or video, or null. */
    fun thumbnail(path: String, size: Int): ByteArray? {
        val f = File(path)
        val s = Size(size, size)
        val bmp = try {
            val name = f.name.lowercase()
            if (listOf(".mp4", ".mkv", ".webm", ".mov", ".3gp").any { name.endsWith(it) }) {
                ThumbnailUtils.createVideoThumbnail(f, s, null)
            } else {
                ThumbnailUtils.createImageThumbnail(f, s, null)
            }
        } catch (e: Exception) {
            return null
        }
        val out = ByteArrayOutputStream()
        bmp.compress(Bitmap.CompressFormat.JPEG, 80, out)
        bmp.recycle()
        return out.toByteArray()
    }

    /** Tells the gallery and other apps about changed files. */
    fun scan(context: Context, paths: List<String>) {
        if (paths.isNotEmpty()) MediaScannerConnection.scanFile(context, paths.toTypedArray(), null, null)
    }

    fun takePhoto(context: Context) {
        if (context.checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
            throw SecurityException("Allow the camera for PALT on the phone first")
        }
        context.startActivity(
            Intent(context, CameraActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_NO_ANIMATION)
        )
    }

    // ── Storage and memory ──────────────────────────────────────────────────

    /**
     * Everything about the phone's space: storage totals (as Settings shows
     * them), what the files are, the biggest folders and files, other
     * volumes (SD card), RAM and swap. Files come from the media index.
     */
    fun storageInfo(context: Context): Map<String, Any?> {
        val stats = context.getSystemService(StorageStatsManager::class.java)
        val total = stats.getTotalBytes(StorageManager.UUID_DEFAULT)
        val free = stats.getFreeBytes(StorageManager.UUID_DEFAULT)

        val root = Environment.getExternalStorageDirectory().absolutePath
        var images = 0L; var videos = 0L; var audio = 0L; var documents = 0L; var other = 0L
        val folders = HashMap<String, Long>()
        val largest = java.util.PriorityQueue<Pair<String, Long>>(compareBy { it.second })
        val docExt = setOf("pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "odt", "ods", "odp", "csv", "epub", "md")
        try {
            context.contentResolver.query(
                MediaStore.Files.getContentUri(MediaStore.VOLUME_EXTERNAL),
                arrayOf(MediaStore.Files.FileColumns.DATA, MediaStore.Files.FileColumns.SIZE, MediaStore.Files.FileColumns.MEDIA_TYPE),
                "${MediaStore.Files.FileColumns.SIZE} > 0", null, null,
            )?.use { c ->
                while (c.moveToNext()) {
                    val path = c.getString(0) ?: continue
                    if (!path.startsWith("$root/")) continue
                    val size = c.getLong(1)
                    when (c.getInt(2)) {
                        MediaStore.Files.FileColumns.MEDIA_TYPE_IMAGE -> images += size
                        MediaStore.Files.FileColumns.MEDIA_TYPE_VIDEO -> videos += size
                        MediaStore.Files.FileColumns.MEDIA_TYPE_AUDIO -> audio += size
                        else -> if (path.substringAfterLast('.', "").lowercase() in docExt) documents += size else other += size
                    }
                    val top = path.substring(root.length + 1).substringBefore('/', "")
                    if (top.isNotEmpty()) folders[top] = (folders[top] ?: 0) + size
                    largest.add(path to size)
                    if (largest.size > 12) largest.poll()
                }
            }
        } catch (_: Exception) {
        }
        val files = images + videos + audio + documents + other

        val am = context.getSystemService(ActivityManager::class.java)
        val mem = ActivityManager.MemoryInfo().also { am.getMemoryInfo(it) }
        val meminfo = try {
            File("/proc/meminfo").readLines().associate { l ->
                l.substringBefore(':') to (l.substringAfter(':').trim().substringBefore(' ').toLongOrNull() ?: 0L) * 1024
            }
        } catch (_: Exception) {
            emptyMap()
        }

        val volumes = context.getSystemService(StorageManager::class.java).storageVolumes
            .filter { !it.isPrimary && it.directory != null }
            .map { v ->
                val st = StatFs(v.directory!!.absolutePath)
                mapOf(
                    "name" to (v.getDescription(context) ?: "Storage"),
                    "path" to v.directory!!.absolutePath,
                    "removable" to v.isRemovable,
                    "total" to st.totalBytes,
                    "free" to st.availableBytes,
                )
            }

        return mapOf(
            "total" to total,
            "free" to free,
            "categories" to mapOf(
                "images" to images, "videos" to videos, "audio" to audio,
                "documents" to documents, "other" to other,
                "system" to (total - free - files).coerceAtLeast(0),
            ),
            "folders" to folders.entries.sortedByDescending { it.value }.take(10)
                .map { mapOf("name" to it.key, "path" to "$root/${it.key}", "size" to it.value) },
            "largest" to largest.sortedByDescending { it.second }
                .map { mapOf("path" to it.first, "size" to it.second) },
            "volumes" to volumes,
            "ram" to mapOf(
                "total" to mem.totalMem,
                "available" to mem.availMem,
                "low" to mem.lowMemory,
                "threshold" to mem.threshold,
                "cached" to (meminfo["Cached"] ?: 0L),
                "swapTotal" to (meminfo["SwapTotal"] ?: 0L),
                "swapFree" to (meminfo["SwapFree"] ?: 0L),
            ),
        )
    }

    // ── Screenshots ─────────────────────────────────────────────────────────

    private var observer: ContentObserver? = null
    private var lastShot = ""

    fun watchScreenshots(context: Context) {
        if (observer != null) return
        val app = context.applicationContext
        val o = object : ContentObserver(Handler(Looper.getMainLooper())) {
            override fun onChange(selfChange: Boolean, uri: Uri?) {
                val path = newestScreenshot(app) ?: return
                if (path == lastShot) return
                lastShot = path
                onScreenshot?.invoke(path)
            }
        }
        app.contentResolver.registerContentObserver(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, true, o)
        observer = o
    }

    private fun newestScreenshot(context: Context): String? {
        val since = System.currentTimeMillis() / 1000 - 20
        return try {
            context.contentResolver.query(
                MediaStore.Images.Media.EXTERNAL_CONTENT_URI,
                arrayOf(MediaStore.Images.Media.DATA, MediaStore.Images.Media.RELATIVE_PATH),
                "${MediaStore.Images.Media.DATE_ADDED} >= ? AND ${MediaStore.Images.Media.IS_PENDING} = 0",
                arrayOf(since.toString()),
                "${MediaStore.Images.Media.DATE_ADDED} DESC",
            )?.use { c ->
                if (!c.moveToFirst()) return null
                val rel = c.getString(1) ?: ""
                if (!rel.contains("Screenshot", ignoreCase = true)) return null
                c.getString(0)
            }
        } catch (e: Exception) {
            null
        }
    }

    /** Offers a Wi-Fi QR screenshot to computers: one button each. */
    fun offerWifi(context: Context, path: String, ssid: String, computers: List<Map<String, String>>) {
        Notifier.ensureChannels(context)
        val b = Notification.Builder(context, Notifier.CHANNEL_MESSAGES)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle("Wi-Fi QR code: $ssid")
            .setContentText("Send it so your computer can join")
            .setAutoCancel(true)
        computers.take(3).forEachIndexed { i, c ->
            val pi = PendingIntent.getBroadcast(
                context, WIFI_NOTIFICATION + i,
                Intent(context, ActionReceiver::class.java).setAction(SEND_WIFI)
                    .putExtra(EXTRA_PATH, path).putExtra(EXTRA_COMPUTER, c["id"]),
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )
            b.addAction(Notification.Action.Builder(null, "Send to ${c["name"]}", pi).build())
        }
        context.getSystemService(NotificationManager::class.java).notify(WIFI_NOTIFICATION, b.build())
    }

    fun sendWifi(context: Context, path: String, computer: String) {
        context.getSystemService(NotificationManager::class.java).cancel(WIFI_NOTIFICATION)
        onSendWifi?.invoke(path, computer)
    }
}

/** Opens the camera for a computer's "Take photo"; saves to Pictures/PALT. */
@TargetApi(Build.VERSION_CODES.R)
class CameraActivity : Activity() {
    private var uri: Uri? = null
    private var name = ""

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        overridePendingTransition(0, 0)
        if (savedInstanceState != null) {
            uri = savedInstanceState.getParcelable("uri")
            name = savedInstanceState.getString("name") ?: ""
            return
        }
        name = "PALT_" + SimpleDateFormat("yyyyMMdd_HHmmss", Locale.US).format(Date()) + ".jpg"
        val values = ContentValues().apply {
            put(MediaStore.Images.Media.DISPLAY_NAME, name)
            put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg")
            put(MediaStore.Images.Media.RELATIVE_PATH, "${Environment.DIRECTORY_PICTURES}/PALT")
        }
        uri = contentResolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
        if (uri == null) {
            done(null)
            return
        }
        try {
            startActivityForResult(
                Intent(MediaStore.ACTION_IMAGE_CAPTURE)
                    .putExtra(MediaStore.EXTRA_OUTPUT, uri)
                    .addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION),
                1,
            )
        } catch (e: Exception) {
            contentResolver.delete(uri!!, null, null)
            done(null)
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        outState.putParcelable("uri", uri)
        outState.putString("name", name)
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        val u = uri ?: return done(null)
        if (resultCode != RESULT_OK) {
            contentResolver.delete(u, null, null)
            return done(null)
        }
        val dir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_PICTURES)
        done(File(File(dir, "PALT"), name).absolutePath)
    }

    private fun done(path: String?) {
        PhoneFiles.onPhoto?.invoke(path)
        finish()
    }
}
