/// pairing.dart — the one secret all my devices share, and the code used to
/// hand it to a new device.
///
///   palt2:<base64url 32-byte key>               LAN only
///   palt2:<base64url 32-byte key>@<host:port>   plus a relay for remote use
///
/// palt1 relay codes are still accepted: their key becomes the pairing key.
/// Must stay identical to desktop/internal/pairing/pairing.go.
library;

import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import '../relay/relay_code.dart';
import 'keys.dart';

const _codePrefix = 'palt2:';

class Pairing {
  final String deviceId;
  final Uint8List key; // 32 bytes shared by all paired devices
  final String relay; // "host:port", empty = LAN only

  const Pairing(this.deviceId, this.key, {this.relay = ''});

  /// A fresh identity in a group of its own.
  factory Pairing.fresh({String? deviceId}) => Pairing(deviceId ?? randomHex(8), newPairingKey());

  GroupKeys get keys => deriveKeys(key);

  /// What another device needs to join this group.
  String get code =>
      '$_codePrefix${base64Url.encode(key).replaceAll('=', '')}${relay.isEmpty ? '' : '@$relay'}';

  /// The relay client's view of the group, if a relay is set.
  RelayCode? get relayCode {
    if (relay.isEmpty) return null;
    final i = relay.lastIndexOf(':');
    if (i <= 0) return null;
    final port = int.tryParse(relay.substring(i + 1));
    if (port == null) return null;
    var host = relay.substring(0, i);
    if (host.startsWith('[') && host.endsWith(']')) host = host.substring(1, host.length - 1);
    return RelayCode(host, port, key);
  }

  Pairing copyWith({Uint8List? key, String? relay}) =>
      Pairing(deviceId, key ?? this.key, relay: relay ?? this.relay);

  Map<String, dynamic> toJson() => {
        'deviceId': deviceId,
        'key': base64.encode(key),
        if (relay.isNotEmpty) 'relay': relay,
      };

  static Pairing? fromJson(Map<String, dynamic> j) {
    try {
      final key = base64.decode(j['key'] as String);
      final id = j['deviceId'] as String;
      if (key.length != 32 || id.isEmpty) return null;
      return Pairing(id, key, relay: j['relay'] as String? ?? '');
    } catch (_) {
      return null;
    }
  }
}

Uint8List newPairingKey() {
  final r = Random.secure();
  return Uint8List.fromList(List.generate(32, (_) => r.nextInt(256)));
}

/// Reads a palt2 or palt1 code. Throws [FormatException] with a readable message.
({Uint8List key, String relay}) parsePairingCode(String input) {
  final s = input.trim();
  if (s.startsWith('palt1:')) {
    final c = RelayCode.parse(s);
    return (key: c.key, relay: c.address);
  }
  if (!s.startsWith(_codePrefix)) {
    throw const FormatException('Pairing code must start with palt2:');
  }
  var rest = s.substring(_codePrefix.length);
  var relay = '';
  final at = rest.indexOf('@');
  if (at >= 0) {
    relay = normalizeRelay(rest.substring(at + 1));
    rest = rest.substring(0, at);
  }
  Uint8List key;
  try {
    key = base64Url.decode(base64Url.normalize(rest));
  } catch (_) {
    throw const FormatException('Pairing code has a bad key');
  }
  if (key.length != 32) throw const FormatException('Pairing code has a bad key');
  return (key: key, relay: relay);
}

/// Turns "host" or "host:port" into "host:port"; empty stays empty.
String normalizeRelay(String addr) {
  final a = addr.trim();
  if (a.isEmpty) return '';
  return RelayCode.generate(a).address;
}
