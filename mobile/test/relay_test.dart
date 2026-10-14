import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:palt_mobile/services/palt_conn.dart';
import 'package:palt_mobile/services/relay/relay_client.dart';
import 'package:palt_mobile/services/relay/relay_code.dart';

void main() {
  test('key vector matches desktop/internal/relay', () {
    final code = RelayCode('h', 1, Uint8List.fromList(List.generate(32, (i) => i)));
    final k = code.keys;
    expect(k.groupId, '523da52e979666af9cf4525abf8fd282');
    expect(k.presenceSig('aaaaaaaaaaaaaaaa', 'laptop', 'linux'),
        '87f067807a42146f8032c9714ef4eb84afaad7996f208cdd4d8ed793554bdd53');
  });

  test('pairing code round trip', () {
    final code = RelayCode('203.0.113.7', 9877, Uint8List.fromList(List.generate(32, (i) => 255 - i)));
    final p = RelayCode.parse(code.toString());
    expect(p.host, code.host);
    expect(p.port, code.port);
    expect(p.key, code.key);
    expect(() => RelayCode.parse('palt1:host:1:short'), throwsFormatException);
    expect(() => RelayCode.parse('nope'), throwsFormatException);
  });

  test('ByteReader reads exact sizes and lines across chunks', () async {
    final ctl = StreamController<Uint8List>();
    final r = ByteReader(ctl.stream);
    ctl.add(Uint8List.fromList('{"t":'.codeUnits));
    ctl.add(Uint8List.fromList('"ok"}\nAB'.codeUnits));
    ctl.add(Uint8List.fromList('CDE'.codeUnits));
    unawaited(ctl.close());
    expect(String.fromCharCodes((await r.readLine())!), '{"t":"ok"}');
    expect(String.fromCharCodes((await r.read(4))!), 'ABCD');
    expect(String.fromCharCodes((await r.read(1))!), 'E');
    expect(await r.read(1), isNull);
  });

  // Run by desktop/internal/relay TestDartInterop, which starts a relay and a
  // Go device named "laptop" and passes the pairing code in the environment.
  final interopCode = Platform.environment['PALT_INTEROP_CODE'];
  test('interop with the Go client through a real relay', () async {
    final code = RelayCode.parse(interopCode!);
    final client = RelayClient(code, 'bbbbbbbbbbbbbbbb', 'phone', 'android');

    // 1. Go dials us: read everything, answer with its SHA-256.
    final incomingDone = Completer<void>();
    client.onIncoming = (conn, from) async {
      expect(from.name, 'laptop');
      final all = BytesBuilder();
      await for (final chunk in conn.input) {
        all.add(chunk);
      }
      conn.add(sha256.convert(all.takeBytes()).bytes);
      await conn.close();
      incomingDone.complete();
    };
    client.start();
    await incomingDone.future.timeout(const Duration(seconds: 20));

    // 2. We dial Go: send 3 MB, expect its SHA-256 back.
    final laptop = client.peerByName('laptop')!;
    final conn = await client.dial(laptop.id);
    final payload = Uint8List.fromList(List.generate(3 * 1024 * 1024 + 11, (i) => (i * 7) & 0xff));
    final reply = BytesBuilder();
    final got = Completer<void>();
    conn.input.listen(reply.add, onDone: got.complete);
    await conn.addStream(Stream.fromIterable([
      for (var o = 0; o < payload.length; o += 100000)
        payload.sublist(o, (o + 100000).clamp(0, payload.length))
    ]));
    await conn.flush();
    // Half-close is not exposed; Go reads exactly len(payload) bytes.
    await got.future.timeout(const Duration(seconds: 20));
    expect(reply.takeBytes(), sha256.convert(payload).bytes);
    conn.destroy();
    client.stop();
  }, skip: interopCode == null ? 'set PALT_INTEROP_CODE (run via Go TestDartInterop)' : false);
}
