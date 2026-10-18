/// background.dart — the Android side that keeps PALT running with the
/// window closed (android/.../PaltService.kt, Channels.kt).
///
/// The Dart engine lives as long as the process; a foreground service keeps
/// the process alive and shows who is connected. Network changes come back
/// from Android so discovery and links react at once.
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../providers/peer_provider.dart';
import '../providers/relay_provider.dart';
import '../features/calls.dart';
import '../features/clipboard.dart';
import '../features/media.dart';
import '../features/messages.dart';
import '../features/notification_sync.dart';
import '../features/phone_features.dart';
import '../features/unlock.dart';
import '../services/link/link.dart';
import '../services/palt_conn.dart';
import 'transfer_service.dart';
import 'usb_path.dart';

const _channel = MethodChannel('palt/service');

class Background {
  static Future<bool> isIgnoringBatteryOptimizations() async {
    try {
      return await _channel.invokeMethod<bool>('isIgnoringBatteryOptimizations') ?? true;
    } catch (_) {
      return true;
    }
  }

  /// Opens Android's "Allow PALT to always run in the background?" prompt.
  static Future<void> requestIgnoreBatteryOptimizations() async {
    try {
      await _channel.invokeMethod('requestIgnoreBatteryOptimizations');
    } catch (_) {}
  }

  static Future<void> _setStatus(String text) async {
    try {
      await _channel.invokeMethod('setStatus', {'text': text});
    } catch (_) {}
  }
}

/// Starts everything that must run with or without a window: the transfer
/// server, discovery, the relay and the link connector. Call once per
/// process, right after the provider container exists.
void startCore(ProviderContainer c) {
  c.read(transferServiceProvider);
  c.read(discoveryServiceProvider);
  c.read(relayClientProvider);
  c.read(connectorProvider);
  unawaited(refreshUsbPrefixes());
  setupPhoneFeatures(c);
  setupNotificationSync(c);
  setupCalls(c);
  setupClipboard(c);
  setupMedia(c);
  setupMessages(c);
  setupUnlock(c);

  // Keep the notification saying who is connected.
  void showLinks(List<Link> links) {
    final names = links.map((l) => l.peer.name).toList()..sort();
    Background._setStatus(names.isEmpty ? 'Looking for your devices' : 'Connected to ${names.join(', ')}');
  }

  c.listen<AsyncValue<List<Link>>>(linksProvider, (_, next) {
    final links = next.valueOrNull;
    if (links != null) showLinks(links);
  }, fireImmediately: true);

  _channel.setMethodCallHandler((call) async {
    if (call.method == 'statusChanged') refreshPhoneStatus();
    if (call.method == 'networkChanged') {
      c.read(discoveryServiceProvider).rescan();
      unawaited(refreshUsbPrefixes());
      (await c.read(relayClientProvider.future))?.reconnectNow();
      // Drop direct links that died with the old network, so the connector
      // moves to the new path now rather than after the read timeout.
      await Future.wait([
        for (final l in c.read(linkManagerProvider).links)
          if (l.route == Route.direct)
            l.probe().then((ok) {
              if (!ok) l.close();
            }),
      ]);
      c.read(connectorProvider).kick();
      refreshPhoneStatus();
    }
  });
}
