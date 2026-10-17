/// clipboard.dart — clipboard sync with paired computers.
///
/// Text copied on a computer lands on this phone's clipboard right away.
/// Android only lets the app on screen read the clipboard, so this phone's
/// clipboard goes out when PALT is opened, from the Quick Settings tile and
/// from the "Send clipboard" button on PALT's notification.
///
/// Packet types (must match desktop/clipboard_app.go):
///   clipboard  either way  {text}
library;

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../providers/device_features_provider.dart';
import '../providers/link_provider.dart';
import '../providers/trust_provider.dart';

const _channel = MethodChannel('palt/clipboard');
const _sendKey = 'palt_clip_send';
const _receiveKey = 'palt_clip_receive';
const _maxText = 256 * 1024;

class ClipboardSettings {
  final bool send;
  final bool receive;
  const ClipboardSettings(this.send, this.receive);
}

final clipboardSettingsProvider = StateNotifierProvider<ClipboardSettingsNotifier, ClipboardSettings>((ref) {
  return ClipboardSettingsNotifier(ref.read(sharedPreferencesProvider));
});

class ClipboardSettingsNotifier extends StateNotifier<ClipboardSettings> {
  final SharedPreferences _p;
  ClipboardSettingsNotifier(this._p)
      : super(ClipboardSettings(_p.getBool(_sendKey) ?? true, _p.getBool(_receiveKey) ?? true));

  void setSend(bool on) {
    _p.setBool(_sendKey, on);
    state = ClipboardSettings(on, state.receive);
  }

  void setReceive(bool on) {
    _p.setBool(_receiveKey, on);
    state = ClipboardSettings(state.send, on);
  }
}

/// Recent clipboard texts (newest first) with where they came from.
final clipboardHistoryProvider = StateProvider<List<({String text, String from, DateTime time})>>((ref) => const []);

void setupClipboard(ProviderContainer c) {
  final links = c.read(linkManagerProvider);
  var last = '';

  void record(String text, String from) {
    c.read(clipboardHistoryProvider.notifier).update((h) => [
          (text: text, from: from, time: DateTime.now()),
          ...h.where((e) => e.text != text),
        ].take(30).toList());
  }

  void send(String text) {
    if (text.isEmpty || text.length > _maxText || text == last) return;
    if (!c.read(clipboardSettingsProvider).send) return;
    last = text;
    record(text, '');
    final features = c.read(deviceFeaturesProvider.notifier);
    links.broadcast('clipboard',
        body: {'text': text}, only: (l) => features.isOn(DeviceFeature.clipboardSend, l.peer.name));
  }

  links.handle('clipboard', (l, p) async {
    final text = p.body?['text'] as String? ?? '';
    if (text.isEmpty || text.length > _maxText || text == last) return;
    if (!c.read(clipboardSettingsProvider).receive) return;
    if (!c.read(deviceFeaturesProvider.notifier).isOn(DeviceFeature.clipboardReceive, l.peer.name)) return;
    last = text;
    record(text, l.peer.name);
    try {
      await _channel.invokeMethod('set', {'text': text});
    } catch (_) {}
  });

  _channel.setMethodCallHandler((call) async {
    if (call.method == 'send') send(call.arguments as String? ?? '');
  });

  // Opening PALT sends whatever was copied since.
  AppLifecycleListener(onResume: () async {
    try {
      final text = await _channel.invokeMethod<String>('get');
      if (text != null) send(text);
    } catch (_) {}
  });
}
