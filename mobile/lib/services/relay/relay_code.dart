/// relay_code.dart — the 1.x relay pairing code ("palt1:<host>:<port>:<key>").
///
/// Still accepted when pairing, so devices paired through the relay stay
/// paired. Must stay identical to desktop/internal/relay/code.go.
library;

import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import '../secure/keys.dart';

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

  GroupKeys get keys => deriveKeys(key);
}

