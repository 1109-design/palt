import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'trust_provider.dart';

/// Per-device feature switches, all on unless turned off for a device:
/// e.g. a work laptop that shouldn't get this phone's clipboard.
class DeviceFeature {
  static const clipboardSend = 'clipboard.send'; // this phone's copies go to it
  static const clipboardReceive = 'clipboard.receive'; // its copies land here
  static const notifications = 'notifications'; // this phone's notifications go to it
  static const mediaControls = 'media.controls'; // its player shows in this phone's media controls

  static const all = [clipboardSend, clipboardReceive, notifications, mediaControls];
}

/// Feature → device names it is turned off for.
final deviceFeaturesProvider = StateNotifierProvider<DeviceFeaturesNotifier, Map<String, Set<String>>>((ref) {
  return DeviceFeaturesNotifier(ref.watch(sharedPreferencesProvider));
});

class DeviceFeaturesNotifier extends StateNotifier<Map<String, Set<String>>> {
  static String _key(String feature) => 'palt_feature_off_$feature';
  final SharedPreferences _prefs;

  DeviceFeaturesNotifier(this._prefs)
      : super({for (final f in DeviceFeature.all) f: (_prefs.getStringList(_key(f)) ?? const []).toSet()});

  bool isOn(String feature, String device) => !(state[feature]?.contains(device) ?? false);

  void set(String feature, String device, bool on) {
    final off = {...?state[feature]};
    on ? off.remove(device) : off.add(device);
    state = {...state, feature: off};
    _prefs.setStringList(_key(feature), off.toList());
  }
}
