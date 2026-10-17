package com.lads.palt.palt_mobile

import android.app.Application
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.FlutterEngineCache
import io.flutter.embedding.engine.dart.DartExecutor

/**
 * Owns the one Flutter engine of the process. The app's Dart code (discovery,
 * links, transfers) runs in it whether or not a window is open: the activity
 * attaches to it when shown, and [PaltService] keeps the process alive when
 * it isn't. Closing the app's window therefore doesn't disconnect PALT.
 */
class PaltApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        val engine = FlutterEngine(this)
        Channels.register(this, engine)
        engine.dartExecutor.executeDartEntrypoint(DartExecutor.DartEntrypoint.createDefault())
        FlutterEngineCache.getInstance().put(ENGINE_ID, engine)
    }

    companion object {
        const val ENGINE_ID = "palt"
    }
}
