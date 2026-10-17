/// phone_features.dart — the everyday live features over links: battery and
/// signal reports, find my phone, pings, and notifications for messages.
///
/// Packet types (must match desktop/features_app.go):
///   battery      phone → desktop  {level 0-100, charging}
///   connectivity phone → desktop  {network, signal 0-4 / -1}
///   ring         either way       {} ring loudly to be found
///   ring.stop    either way       {}
///   user_ping    either way       {message?}
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../services/link/link.dart';
import '../services/transfer_service.dart';

const _device = MethodChannel('palt/device');
const _intent = MethodChannel('palt/intent');

/// A share from another app or a notification tap, for the UI to act on:
/// {paths: [..], text: ..} or {openDevice: name}.
final intakeProvider = StateProvider<Map<String, dynamic>?>((ref) => null);

bool get appInForeground => WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;

int _notifId = 100;

/// Shows a phone notification. [url] adds an "Open link" button; tapping
/// opens [device]'s chat.
Future<void> showPhoneNotification(String title, String text, {String? device, String? url}) async {
  try {
    await _device.invokeMethod('notify', {
      'id': _notifId++,
      'title': title,
      'text': text,
      'device': device,
      'url': url,
    });
  } catch (_) {}
}

/// The text as a URL when it is nothing but one web link.
String? singleUrl(String text) {
  final t = text.trim();
  if (t.contains(RegExp(r'\s'))) return null;
  final u = Uri.tryParse(t);
  if (u == null || (u.scheme != 'http' && u.scheme != 'https') || u.host.isEmpty) return null;
  return t;
}

/// Wires the features into the running core. Call once from startCore.
void setupPhoneFeatures(ProviderContainer c) {
  final links = c.read(linkManagerProvider);

  links.handle('ring', (l, p) => _device.invokeMethod('ring', {'from': l.peer.name}));
  links.handle('ring.stop', (l, p) => _device.invokeMethod('stopRing'));
  links.handle('user_ping', (l, p) {
    final msg = (p.body?['message'] as String?) ?? '';
    return showPhoneNotification(l.peer.name, msg.isEmpty ? 'Ping!' : msg, device: l.peer.name);
  });

  // Battery: on every change, and to each device as it links up.
  Map<String, dynamic>? battery;
  Map<String, dynamic>? connectivity;
  Future<void> refreshConnectivity({bool force = false}) async {
    try {
      final now = Map<String, dynamic>.from(await _device.invokeMethod('connectivity') as Map);
      if (force || now.toString() != connectivity.toString()) {
        connectivity = now;
        links.broadcast('connectivity', body: now);
      }
    } catch (_) {}
  }

  _device.setMethodCallHandler((call) async {
    switch (call.method) {
      case 'battery':
        battery = Map<String, dynamic>.from(call.arguments as Map);
        links.broadcast('battery', body: battery);
    }
  });
  _device.invokeMethod('battery').then((b) {
    battery = Map<String, dynamic>.from(b as Map);
  }, onError: (_) {});
  Timer.periodic(const Duration(seconds: 60), (_) => refreshConnectivity());

  final told = <Link>{};
  c.listen<AsyncValue<List<Link>>>(linksProvider, (_, next) {
    final current = next.valueOrNull ?? const <Link>[];
    told.removeWhere((l) => !current.contains(l));
    for (final l in current) {
      if (!told.add(l)) continue;
      if (battery != null && l.supports('battery')) l.send('battery', body: battery).catchError((_) {});
      if (l.supports('connectivity')) {
        refreshConnectivity(force: true);
      }
    }
  });

  // Messages arriving while the app isn't on screen.
  c.listen<IncomingText?>(incomingTextProvider, (_, t) {
    if (t == null || appInForeground) return;
    final url = singleUrl(t.text);
    showPhoneNotification(t.senderName, t.copied ? '${t.text}\n(copied to clipboard)' : t.text,
        device: t.senderName, url: url);
  });
  c.listen<OfferData?>(activeOfferProvider, (_, offer) {
    if (offer == null || appInForeground) return;
    final n = offer.files.length;
    showPhoneNotification(offer.senderName, 'Wants to send you $n file${n == 1 ? '' : 's'} — tap to accept or decline',
        device: offer.senderName);
  });
  c.listen<ReceivedFiles?>(receivedFilesProvider, (_, r) {
    if (r == null || appInForeground) return;
    final n = r.names.length;
    showPhoneNotification(r.senderName, n == 1 ? 'Sent you ${r.names.first}' : 'Sent you $n files',
        device: r.senderName);
  });

  // Shares from other apps and notification taps.
  Future<void> take() async {
    try {
      final m = await _intent.invokeMethod('take');
      if (m != null) c.read(intakeProvider.notifier).state = Map<String, dynamic>.from(m as Map);
    } catch (_) {}
  }

  _intent.setMethodCallHandler((call) async {
    if (call.method == 'ready') await take();
  });
  take();
}
