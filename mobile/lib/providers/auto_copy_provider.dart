import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'trust_provider.dart';

/// Devices whose incoming text messages are copied to the clipboard as they
/// arrive. Only honoured for paired devices (see TransferService).
final autoCopyProvider = StateNotifierProvider<AutoCopyNotifier, List<String>>((ref) {
  final prefs = ref.watch(sharedPreferencesProvider);
  return AutoCopyNotifier(prefs);
});

class AutoCopyNotifier extends StateNotifier<List<String>> {
  static const _key = 'palt_auto_copy_devices';
  final SharedPreferences _prefs;

  AutoCopyNotifier(this._prefs) : super(_prefs.getStringList(_key) ?? []);

  bool isEnabled(String peerName) => state.contains(peerName);

  void set(String peerName, bool enabled) {
    final next = state.where((p) => p != peerName).toList();
    if (enabled) next.add(peerName);
    state = next;
    _prefs.setStringList(_key, state);
  }
}
