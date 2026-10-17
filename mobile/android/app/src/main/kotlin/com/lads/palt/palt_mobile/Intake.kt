package com.lads.palt.palt_mobile

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.OpenableColumns
import java.io.File

/**
 * What the app was opened with: files or text shared from another app
 * ("Share → PALT"), or a notification asking to open a device's chat.
 * Dart takes it with palt/intent.take once its UI is up.
 */
object Intake {
    @Volatile
    var pending: Map<String, Any?>? = null

    /** Called by MainActivity for each intent. Returns true if there is something to hand over. */
    fun accept(context: Context, intent: Intent?): Boolean {
        intent ?: return false
        intent.getStringExtra(MainActivity.EXTRA_OPEN_DEVICE)?.let {
            pending = mapOf("openDevice" to it)
            return true
        }
        val uris = when (intent.action) {
            Intent.ACTION_SEND -> listOfNotNull(stream(intent))
            Intent.ACTION_SEND_MULTIPLE -> streams(intent)
            else -> return false
        }
        val text = if (uris.isEmpty()) intent.getStringExtra(Intent.EXTRA_TEXT) else null
        if (uris.isEmpty() && text.isNullOrBlank()) return false
        pending = mapOf("paths" to copyAll(context, uris), "text" to text)
        return true
    }

    @Suppress("DEPRECATION")
    private fun stream(i: Intent): Uri? =
        if (Build.VERSION.SDK_INT >= 33) i.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
        else i.getParcelableExtra(Intent.EXTRA_STREAM)

    @Suppress("DEPRECATION")
    private fun streams(i: Intent): List<Uri> =
        (if (Build.VERSION.SDK_INT >= 33) i.getParcelableArrayListExtra(Intent.EXTRA_STREAM, Uri::class.java)
        else i.getParcelableArrayListExtra(Intent.EXTRA_STREAM)) ?: emptyList()

    /** Copies shared content into the cache so Dart can read it as files. */
    private fun copyAll(context: Context, uris: List<Uri>): List<String> {
        val dir = File(context.cacheDir, "share")
        dir.deleteRecursively()
        dir.mkdirs()
        val used = mutableSetOf<String>()
        return uris.mapNotNull { uri ->
            try {
                var name = displayName(context, uri) ?: "shared"
                name = name.replace('/', '_')
                var unique = name
                var n = 1
                while (!used.add(unique)) unique = "${n++}-$name"
                val out = File(dir, unique)
                context.contentResolver.openInputStream(uri)?.use { input ->
                    out.outputStream().use { input.copyTo(it) }
                } ?: return@mapNotNull null
                out.absolutePath
            } catch (e: Exception) {
                null
            }
        }
    }

    private fun displayName(context: Context, uri: Uri): String? {
        if (uri.scheme == "file") return uri.lastPathSegment
        context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) return c.getString(0)
        }
        return uri.lastPathSegment
    }
}
