/// relay_code.dart — pairing code and key derivation for the PALT relay.
///
/// Must stay byte-for-byte identical to desktop/internal/relay/code.go.
library;

import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

const _codePrefix = 'palt1:';
const kRelayDefaultPort = 9877;

/// A parsed pairing code: "palt1:<host>:<port>:<base64url key>".
class RelayCode {
  final String host;
  final int port;
  final Uint8List key;

  RelayCode(this.host, this.port, this.key);

  static RelayCode parse(String input) {
    final s = input.trim();
    if (!s.startsWith(_codePrefix)) {
      throw const FormatException('Pairing code must start with palt1:');
    }
    final rest = s.substring(_codePrefix.length);
    final i = rest.lastIndexOf(':');
    if (i < 0) throw const FormatException('Pairing code is incomplete');
    final hostPort = rest.substring(0, i);
    final keyStr = rest.substring(i + 1);

    final j = hostPort.lastIndexOf(':');
    if (j <= 0) throw const FormatException('Pairing code has a bad relay address');
    var host = hostPort.substring(0, j);
    if (host.startsWith('[') && host.endsWith(']')) {
      host = host.substring(1, host.length - 1);
    }
    final port = int.tryParse(hostPort.substring(j + 1));
    if (port == null || port < 1 || port > 65535) {
      throw const FormatException('Pairing code has a bad relay port');
    }
    Uint8List key;
    try {
      key = base64Url.decode(base64Url.normalize(keyStr));
    } catch (_) {
      throw const FormatException('Pairing code has a bad key');
    }
    if (key.length != 32) throw const FormatException('Pairing code has a bad key');
    return RelayCode(host, port, key);
  }

  /// A fresh code for the relay at [addr] ("host" or "host:port").
  static RelayCode generate(String addr) {
    var a = addr.trim();
    if (a.isEmpty) throw const FormatException('Enter the relay server address');
    var host = a;
    var port = kRelayDefaultPort;
    final i = a.lastIndexOf(':');
    if (i > 0 && !a.endsWith(']') && a.indexOf(':') == i) {
      final p = int.tryParse(a.substring(i + 1));
      if (p == null || p < 1 || p > 65535) throw FormatException('Bad port in "$a"');
      host = a.substring(0, i);
      port = p;
    }
    if (host.isEmpty || host.contains(' ') || host.contains('/')) {
      throw FormatException('Bad relay address "$a"');
    }
    final r = Random.secure();
    return RelayCode(host, port, Uint8List.fromList(List.generate(32, (_) => r.nextInt(256))));
  }

  String get address => host.contains(':') ? '[$host]:$port' : '$host:$port';

  @override
  String toString() =>
      '$_codePrefix$address:${base64Url.encode(key).replaceAll('=', '')}';

  RelayKeys get keys {
    final gid = hmacSha256(key, utf8.encode('palt-group-id'));
    return RelayKeys(
      groupId: hex(gid).substring(0, 32),
      authKey: hmacSha256(key, utf8.encode('palt-auth')),
    );
  }
}

class RelayKeys {
  /// Sent to the relay; lets it match devices, reveals nothing else.
  final String groupId;

  /// Never leaves the device.
  final Uint8List authKey;

  const RelayKeys({required this.groupId, required this.authKey});

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
