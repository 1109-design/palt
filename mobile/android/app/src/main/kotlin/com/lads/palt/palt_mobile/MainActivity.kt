package com.lads.palt.palt_mobile

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity

/** The window onto the long-lived engine owned by [PaltApplication]. */
class MainActivity : FlutterActivity() {
    override fun getCachedEngineId(): String = PaltApplication.ENGINE_ID

    // The engine outlives the window: PALT keeps running in the background.
    override fun shouldDestroyEngineWithHost(): Boolean = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // The "Connected" notification needs this on Android 13+.
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1)
        }
        PaltService.start(this)
        handleIntent(intent)
    }

    override fun onResume() {
        super.onResume()
        // Android asks before an app may read other apps' log lines, and only
        // while it is on screen, so the copy watcher starts from here.
        ClipboardWatcher.start(this)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIntent(intent)
    }

    private fun handleIntent(intent: Intent?) {
        Thread {
            if (Intake.accept(applicationContext, intent)) {
                runOnUiThread { Channels.intentReady() }
            }
        }.start()
    }

    companion object {
        const val EXTRA_OPEN_DEVICE = "palt.openDevice"
    }
}
