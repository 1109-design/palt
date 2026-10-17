/// lan_protocol.dart — framing shared by every PALT connection.
///
/// v1 (any device): u32 big-endian length | offer JSON, then a verdict byte.
/// v2 (paired devices) starts with the magic "PALT" instead:
///
///   "PALT" | u8 version (2) | u16 length | hello JSON {"from","nonce"}
///
/// then the secure handshake (session = "palt-lan|" + hello JSON), then the
/// v1 exchange inside the encrypted pipe. Must stay identical to
/// desktop/internal/transfer/protocol.go.
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'palt_conn.dart';
import 'secure/keys.dart';
import 'secure/pipe.dart';

/// Must match MaxOfferSize in desktop/internal/transfer/protocol.go.
const kMaxOfferBytes = 64 * 1024;

const _helloVersion = 2;
const _maxHello = 4096;
const _magic = [0x50, 0x41, 0x4c, 0x54]; // "PALT"

bool isHelloMagic(List<int> first4) =>
    first4.length == 4 &&
    first4[0] == _magic[0] &&
    first4[1] == _magic[1] &&
    first4[2] == _magic[2] &&
    first4[3] == _magic[3];

String _session(List<int> helloJson) => 'palt-lan|${utf8.decode(helloJson)}';

/// Opens an encrypted connection to a paired device on the LAN.
Future<SecureConn> secureDial(
  String ip,
  int port, {
  required String selfId,
  required GroupKeys keys,
  Duration timeout = const Duration(seconds: 4),
}) async {
  final sock = await Socket.connect(ip, port, timeout: timeout);
  try {
    sock.setOption(SocketOption.tcpNoDelay, true);
    final hello = utf8.encode(jsonEncode({'from': selfId, 'nonce': randomHex(16)}));
    final hdr = ByteData(3)
      ..setUint8(0, _helloVersion)
      ..setUint16(1, hello.length);
    sock.add([..._magic, ...hdr.buffer.asUint8List(), ...hello]);
    return await secureHandshake(sock, ByteReader(sock), keys, _session(hello),
        initiator: true, route: Route.direct);
  } catch (_) {
    sock.destroy();
    rethrow;
  }
}

/// Completes a v2 connection whose magic was already read from [reader].
/// Returns the sender's device ID and the encrypted connection.
Future<({String from, SecureConn conn})> secureAccept(
    Socket raw, ByteReader reader, GroupKeys keys) async {
  final hdr = await reader.read(3);
  if (hdr == null) throw const SocketException('connection closed during hello');
  final b = ByteData.sublistView(hdr);
  if (b.getUint8(0) != _helloVersion) {
    throw FormatException('unsupported protocol version ${b.getUint8(0)}');
  }
  final n = b.getUint16(1);
  if (n == 0 || n > _maxHello) throw const FormatException('bad hello size');
  final hello = await reader.read(n);
  if (hello == null) throw const SocketException('connection closed during hello');
  final j = jsonDecode(utf8.decode(hello)) as Map<String, dynamic>;
  final from = j['from'] as String? ?? '';
  if (from.isEmpty) throw const FormatException('bad hello');
  final conn = await secureHandshake(raw, reader, keys, _session(hello),
      initiator: false, route: Route.direct);
  return (from: from, conn: conn);
}

/// Reads an offer frame; [prefix] is its length header when already read.
Future<Map<String, dynamic>?> readOfferJson(ByteReader reader, {Uint8List? prefix}) async {
  final hdr = prefix ?? await reader.read(4);
  if (hdr == null) return null;
  final n = ByteData.sublistView(hdr).getUint32(0);
  if (n > kMaxOfferBytes) throw const FormatException('offer too large');
  final body = await reader.read(n);
  if (body == null) return null;
  return jsonDecode(utf8.decode(body)) as Map<String, dynamic>;
}

/// Writes an offer frame.
void writeOfferJson(PaltConn conn, Map<String, dynamic> offer) {
  final bytes = utf8.encode(jsonEncode(offer));
  final hdr = ByteData(4)..setUint32(0, bytes.length);
  conn.add([...hdr.buffer.asUint8List(), ...bytes]);
}
