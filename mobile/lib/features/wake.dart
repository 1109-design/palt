/// wake.dart — wakes a sleeping computer with a Wake-on-LAN magic packet.
///
/// Computers send their network card's address and LAN broadcast address
/// with their status (desktop/computer_status_linux.go); the phone keeps
/// the last ones so it can wake the computer once it's gone quiet. Only
/// works on the same network and from sleep, not from shut down.
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'phone_features.dart';

const _prefsKey = 'palt_wake_targets';

class WakeTarget {
  final String name, mac, addr;
  final bool ready; // the computer said its card wakes on a magic packet
  const WakeTarget(this.name, this.mac, this.addr, this.ready);

  Map<String, dynamic> toJson() => {'name': name, 'mac': mac, 'addr': addr, 'ready': ready};
  factory WakeTarget.fromJson(Map<String, dynamic> j) =>
      WakeTarget(j['name'] as String, j['mac'] as String, j['addr'] as String? ?? '', j['ready'] as bool? ?? false);
}

class WakeTargets extends StateNotifier<Map<String, WakeTarget>> {
  WakeTargets() : super(const {}) {
    _load();
  }

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    try {
      final m = jsonDecode(prefs.getString(_prefsKey) ?? '{}') as Map<String, dynamic>;
      state = {...m.map((k, v) => MapEntry(k, WakeTarget.fromJson(v as Map<String, dynamic>))), ...state};
    } catch (_) {}
  }

  Future<void> remember(String name, DeviceStatus s) async {
    final t = WakeTarget(name, s.wakeMac, s.wakeAddr, s.wakeReady);
    final old = state[name];
    if (old != null && old.mac == t.mac && old.addr == t.addr && old.ready == t.ready) return;
    state = {...state, name: t};
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefsKey, jsonEncode(state.map((k, v) => MapEntry(k, v.toJson()))));
  }
}

final wakeTargetsProvider = StateNotifierProvider<WakeTargets, Map<String, WakeTarget>>((ref) => WakeTargets());

/// Sends the magic packet a few times to the LAN and global broadcast.
Future<void> wake(WakeTarget t) async {
  final mac = [for (final h in t.mac.split(':')) int.parse(h, radix: 16)];
  if (mac.length != 6) throw 'bad address ${t.mac}';
  final packet = Uint8List(102)..fillRange(0, 6, 0xFF);
  for (var i = 0; i < 16; i++) {
    packet.setRange(6 + i * 6, 12 + i * 6, mac);
  }
  final sock = await RawDatagramSocket.bind(InternetAddress.anyIPv4, 0);
  sock.broadcastEnabled = true;
  try {
    for (var round = 0; round < 3; round++) {
      for (final addr in {if (t.addr.isNotEmpty) t.addr, '255.255.255.255'}) {
        for (final port in const [9, 7]) {
          sock.send(packet, InternetAddress(addr), port);
        }
      }
      await Future<void>.delayed(const Duration(milliseconds: 300));
    }
  } finally {
    sock.close();
  }
}
