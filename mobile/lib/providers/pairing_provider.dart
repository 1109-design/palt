/// pairing_provider.dart — this device's identity and pairing key.
///
/// Every install belongs to one group; a fresh install is a group of one.
/// Scanning or pasting another device's code joins its group. A 1.x relay
/// code is migrated into the pairing, so devices paired through the relay
/// stay paired.
library;

import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../services/relay/relay_code.dart';
import '../services/secure/keys.dart';
import '../services/secure/pairing.dart';
import 'trust_provider.dart';

const _pairingKey = 'palt_pairing';
const _oldCodeKey = 'palt_relay_code';
const _oldDeviceIdKey = 'palt_relay_device_id';

final pairingProvider = StateNotifierProvider<PairingNotifier, Pairing>((ref) {
  return PairingNotifier(ref.read(sharedPreferencesProvider));
});

class PairingNotifier extends StateNotifier<Pairing> {
  final SharedPreferences _prefs;

  PairingNotifier(this._prefs) : super(_load(_prefs));

  static Pairing _load(SharedPreferences prefs) {
    final saved = prefs.getString(_pairingKey);
    if (saved != null) {
      try {
        final p = Pairing.fromJson(jsonDecode(saved) as Map<String, dynamic>);
        if (p != null) return p;
      } catch (_) {}
    }
    var p = Pairing.fresh(deviceId: prefs.getString(_oldDeviceIdKey) ?? randomHex(8));
    final old = prefs.getString(_oldCodeKey);
    if (old != null) {
      try {
        final c = RelayCode.parse(old);
        p = p.copyWith(key: c.key, relay: c.address);
      } catch (_) {}
    }
    prefs.setString(_pairingKey, jsonEncode(p.toJson()));
    return p;
  }

  Future<void> _save(Pairing p) async {
    await _prefs.setString(_pairingKey, jsonEncode(p.toJson()));
    state = p;
  }

  /// Joins the group of the device that showed [code]. Throws
  /// [FormatException] with a readable message for bad codes.
  Future<void> join(String code) async {
    final parsed = parsePairingCode(code);
    await _save(state.copyWith(
      key: parsed.key,
      relay: parsed.relay.isEmpty ? null : parsed.relay,
    ));
  }

  /// Starts a new group: every other device is unpaired.
  Future<void> reset() => _save(state.copyWith(key: newPairingKey()));

  /// Sets the relay ("host" or "host:port"); empty turns it off.
  Future<void> setRelay(String addr) => _save(state.copyWith(relay: normalizeRelay(addr)));
}
