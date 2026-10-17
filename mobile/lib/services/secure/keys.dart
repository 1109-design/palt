/// keys.dart — what the pairing key shared by all my devices expands into.
///
/// Must stay byte-for-byte identical to desktop/internal/secure/keys.go.
library;

import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

/// Expands the 32-byte pairing key.
GroupKeys deriveKeys(List<int> pairingKey) {
  final gid = hmacSha256(pairingKey, utf8.encode('palt-group-id'));
  return GroupKeys(
    groupId: hex(gid).substring(0, 32),
    authKey: hmacSha256(pairingKey, utf8.encode('palt-auth')),
  );
}

class GroupKeys {
  /// Sent to the relay; lets it match devices, reveals nothing else.
  final String groupId;

  /// Never leaves the device.
  final Uint8List authKey;

  const GroupKeys({required this.groupId, required this.authKey});

  /// Authenticates a presence entry so the relay can't inject devices.
  String presenceSig(String id, String name, String os) =>
      hex(hmacSha256(authKey, utf8.encode('palt-presence|$id|$name|$os')));

  bool verifyPresence(String id, String name, String os, String sig) =>
      constantTimeEquals(utf8.encode(presenceSig(id, name, os)), utf8.encode(sig));
}

Uint8List hmacSha256(List<int> key, List<int> data) =>
    Uint8List.fromList(Hmac(sha256, key).convert(data).bytes);

/// HMAC over the concatenation of [parts].
Uint8List hmacParts(List<int> key, List<List<int>> parts) {
  final out = _DigestSink();
  final input = Hmac(sha256, key).startChunkedConversion(out);
  for (final p in parts) {
    input.add(p);
  }
  input.close();
  return Uint8List.fromList(out.value.bytes);
}

class _DigestSink implements Sink<Digest> {
  late Digest value;
  @override
  void add(Digest data) => value = data;
  @override
  void close() {}
}

/// HKDF-SHA256 (RFC 5869).
Uint8List hkdfSha256(List<int> ikm, List<int> salt, List<int> info, int length) {
  final prk = hmacSha256(salt, ikm);
  final out = BytesBuilder();
  var t = <int>[];
  for (var i = 1; out.length < length; i++) {
    t = hmacSha256(prk, [...t, ...info, i]);
    out.add(t);
  }
  return Uint8List.sublistView(out.takeBytes(), 0, length);
}

bool constantTimeEquals(List<int> a, List<int> b) {
  if (a.length != b.length) return false;
  var diff = 0;
  for (var i = 0; i < a.length; i++) {
    diff |= a[i] ^ b[i];
  }
  return diff == 0;
}

String hex(List<int> bytes) =>
    bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();

String randomHex(int nBytes) {
  final r = Random.secure();
  return hex(List<int>.generate(nBytes, (_) => r.nextInt(256)));
}
