import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:palt_mobile/services/lan_protocol.dart';
import 'package:palt_mobile/services/link/link.dart';
import 'package:palt_mobile/services/palt_conn.dart';
import 'package:palt_mobile/services/secure/keys.dart';

void main() {
  test('link packets and duplicate resolution between two Dart managers', () async {
    final keys = deriveKeys(List.generate(32, (i) => i));
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final a = LinkManager(() => const LinkIdentity(id: 'aaaa', name: 'a', os: 'linux'));
    final b = LinkManager(() => const LinkIdentity(id: 'bbbb', name: 'b', os: 'android'));
    final got = Completer<LinkPacket>();
    b.handle('battery', (l, p) => got.complete(p));

    server.listen((s) async {
      final reader = ByteReader(s);
      final first = await reader.read(4);
      expect(isHelloMagic(first!), isTrue);
      final acc = await secureAccept(s, reader, keys);
      expect(acc.from, 'aaaa');
      final r2 = ByteReader(acc.conn.input);
      final offer = await readOfferJson(r2);
      expect(offer!['action'], 'link');
      acc.conn.add([1]);
      await b.run(acc.conn, r2, Route.direct, initiator: false);
    });

    final conn = await secureDial('127.0.0.1', server.port, selfId: 'aaaa', keys: keys);
    final reader = ByteReader(conn.input);
    writeOfferJson(conn, {'action': 'link', 'transferId': 'x', 'files': [], 'totalSize': 0, 'senderName': 'a'});
    expect((await reader.read(1))![0], 1);
    final la = await a.run(conn, reader, Route.direct, initiator: true);
    expect(la.peer.name, 'b');
    expect(la.supports('battery'), isTrue);
    await la.send('battery', body: {'level': 42});
    expect((await got.future.timeout(const Duration(seconds: 5))).body!['level'], 42);

    la.close();
    await Future.delayed(const Duration(milliseconds: 200));
    expect(b.get('aaaa'), isNull);
    await server.close();
  });

  test('wrong pairing key is refused', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((s) async {
      final reader = ByteReader(s);
      await reader.read(4);
      try {
        await secureAccept(s, reader, deriveKeys(List.filled(32, 1)));
      } catch (_) {
        s.destroy();
      }
    });
    await expectLater(
      secureDial('127.0.0.1', server.port, selfId: 'x', keys: deriveKeys(List.filled(32, 2))),
      throwsA(anything),
    );
    await server.close();
  });

  // Run by desktop/internal/link TestDartLANInterop, which starts a Go
  // transfer server and link manager ("laptop") and passes key and port.
  final keyB64 = Platform.environment['PALT_LAN_KEY'];
  test('interop with the Go side over the LAN', () async {
    final keys = deriveKeys(base64.decode(keyB64!));
    final goPort = int.parse(Platform.environment['PALT_LAN_PORT']!);
    const me = 'bbbbbbbbbbbbbbbb';

    // 1. Paired text message.
    final c1 = await secureDial('127.0.0.1', goPort, selfId: me, keys: keys);
    final r1 = ByteReader(c1.input);
    writeOfferJson(c1, {
      'action': 'text', 'transferId': 't', 'files': [], 'totalSize': 0,
      'senderName': 'phone', 'text': 'hello from dart',
    });
    await c1.flush();
    expect((await r1.read(1))![0], 1);
    c1.destroy();

    // Our own server, for Go to dial back.
    final m = LinkManager(() => const LinkIdentity(id: me, name: 'phone', os: 'android'));
    final done = Completer<Link>();
    m.handle('done', (l, p) => done.complete(l));
    m.handle('echo', (l, p) {});
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((s) async {
      final reader = ByteReader(s);
      await reader.read(4);
      final acc = await secureAccept(s, reader, keys);
      final r = ByteReader(acc.conn.input);
      await readOfferJson(r);
      acc.conn.add([1]);
      await m.run(acc.conn, r, Route.direct, initiator: false);
    });

    // 2. Link to Go; echo round trip; tell Go our port.
    final c2 = await secureDial('127.0.0.1', goPort, selfId: me, keys: keys);
    final r2 = ByteReader(c2.input);
    writeOfferJson(c2, {'action': 'link', 'transferId': 'l', 'files': [], 'totalSize': 0, 'senderName': 'phone'});
    await c2.flush();
    expect((await r2.read(1))![0], 1);
    final first = await m.run(c2, r2, Route.direct, initiator: true);
    expect(first.peer.name, 'laptop');
    await first.send('port', body: {'port': server.port});

    // 3. Go dials us; its link (lower ID dialed) replaces ours.
    final kept = await done.future.timeout(const Duration(seconds: 20));
    expect(identical(kept, first), isFalse);
    expect(identical(m.get('aaaaaaaaaaaaaaaa'), kept), isTrue);
    await first.done.timeout(const Duration(seconds: 5));
    kept.close();
    await server.close();
  }, skip: keyB64 == null ? 'set PALT_LAN_KEY (run via Go TestDartLANInterop)' : false);
}
