/// link_provider.dart — keeps a link open to every paired device that can be
/// reached. Mirrors desktop/links_app.go.
///
///   linkManagerProvider → the LinkManager (lives as long as the app process)
///   linksProvider       → live list of open links
///   connectorProvider   → dials paired devices: LAN first, then the relay
library;

import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/discovery_service.dart';
import '../services/lan_protocol.dart';
import '../services/link/link.dart';
import '../services/palt_conn.dart';
import '../services/secure/keys.dart';
import '../services/secure/pipe.dart';
import '../utils/version.dart';
import 'pairing_provider.dart';
import 'peer_provider.dart';
import 'relay_provider.dart';
import 'trust_provider.dart';

const _knownKey = 'palt_known_devices';
const _connectInterval = Duration(seconds: 5);
const _backoffSeen = Duration(seconds: 30);
const _backoffKnown = Duration(minutes: 5);
const _backoffOtherGroup = Duration(minutes: 10);

/// This device's name as peers see it.
final localNameProvider = FutureProvider<String>((ref) => fetchLocalDeviceName());

final linkManagerProvider = Provider<LinkManager>((ref) {
  final m = LinkManager(() => LinkIdentity(
        id: ref.read(pairingProvider).deviceId,
        name: ref.read(localNameProvider).valueOrNull ?? 'Android',
        os: 'android',
        version: kAppVersion,
      ));
  m.onUp = (l) => _remember(ref, l);
  // A new pairing key means a new group: links to the old one must go.
  ref.listen(pairingProvider.select((p) => base64.encode(p.key)), (_, __) => m.closeAll());
  ref.onDispose(m.closeAll);
  return m;
});

final linksProvider = StreamProvider<List<Link>>((ref) async* {
  final m = ref.watch(linkManagerProvider);
  yield m.links;
  yield* m.changes;
});

// ── Known devices ────────────────────────────────────────────────────────────

/// Paired devices seen before: id → {name, os, lastIp}.
Map<String, Map<String, dynamic>> knownDevices(Ref ref) {
  final raw = ref.read(sharedPreferencesProvider).getString(_knownKey);
  if (raw == null) return {};
  try {
    return (jsonDecode(raw) as Map<String, dynamic>)
        .map((k, v) => MapEntry(k, Map<String, dynamic>.from(v as Map)));
  } catch (_) {
    return {};
  }
}

void _remember(Ref ref, Link l) {
  final all = knownDevices(ref);
  final d = all[l.peer.id] ?? {};
  d['name'] = l.peer.name;
  d['os'] = l.peer.os;
  d['lastSeen'] = DateTime.now().toIso8601String();
  final ip = l.remoteIp;
  if (ip != null) d['lastIp'] = ip;
  all[l.peer.id] = d;
  ref.read(sharedPreferencesProvider).setString(_knownKey, jsonEncode(all));
}

// ── Connector ────────────────────────────────────────────────────────────────

final connectorProvider = Provider<Connector>((ref) {
  final c = Connector(ref);
  c.start();
  // Anything new on the LAN or the relay: try right away.
  ref.listen(lanPeerListProvider, (_, __) => c.kick());
  ref.listen(relayPeersProvider, (_, __) => c.kick());
  ref.onDispose(c.stop);
  return c;
});

class _Candidate {
  String? lanIp;
  int lanPort = kPaltPort;
  bool seen = false; // the LAN address is a live sighting, not memory
  bool relay = false;
}

class _State {
  DateTime next = DateTime.fromMillisecondsSinceEpoch(0);
  int failures = 0;
  bool busy = false;
}

class Connector {
  final Ref ref;
  final _states = <String, _State>{};
  Timer? _timer;

  Connector(this.ref);

  /// Starts dialing once this device's name is known, so peers never see
  /// a placeholder name.
  Future<void> start() async {
    try {
      await ref.read(localNameProvider.future);
    } catch (_) {}
    _timer = Timer.periodic(_connectInterval, (_) => _tick());
    _tick();
  }

  void stop() => _timer?.cancel();

  /// Something changed (network, relay, discovery): retry right away.
  void kick() {
    for (final s in _states.values) {
      if (s.failures < 3) s.next = DateTime.fromMillisecondsSinceEpoch(0);
    }
    _tick();
  }

  void _tick() {
    final self = ref.read(pairingProvider).deviceId;
    final links = ref.read(linkManagerProvider);
    final cands = <String, _Candidate>{};
    _Candidate get(String id) => cands.putIfAbsent(id, _Candidate.new);

    for (final p in ref.read(lanPeerListProvider).valueOrNull ?? const []) {
      if (p.deviceId.isNotEmpty && p.deviceId != self) {
        get(p.deviceId)
          ..lanIp = p.ipAddress
          ..lanPort = p.port
          ..seen = true;
      }
    }
    for (final r in ref.read(relayPeersProvider).valueOrNull ?? const []) {
      get(r.id).relay = true;
    }
    knownDevices(ref).forEach((id, d) {
      final ip = d['lastIp'] as String?;
      if (ip != null && id != self) {
        final c = get(id);
        c.lanIp ??= ip;
      }
    });

    final now = DateTime.now();
    cands.forEach((id, c) {
      final l = links.get(id);
      if (l != null && (l.route == Route.direct || c.lanIp == null)) return;
      final st = _states.putIfAbsent(id, _State.new);
      if (st.busy || now.isBefore(st.next)) return;
      st.busy = true;
      _connect(id, c, haveLink: l != null).then((_) {
        st.failures = 0;
        st.next = DateTime.fromMillisecondsSinceEpoch(0);
      }, onError: (Object e) {
        st.failures++;
        final limit = c.seen ? _backoffSeen : _backoffKnown;
        var wait = _connectInterval * (1 << st.failures.clamp(0, 8));
        if (wait > limit) wait = limit;
        if (e is HandshakeException) wait = _backoffOtherGroup;
        st.next = DateTime.now().add(wait);
      }).whenComplete(() => st.busy = false);
    });
  }

  Future<void> _connect(String id, _Candidate c, {required bool haveLink}) async {
    final p = ref.read(pairingProvider);
    Object? lanErr;
    if (c.lanIp != null) {
      try {
        await openLink(
            () => secureDial(c.lanIp!, c.lanPort, selfId: p.deviceId, keys: p.keys), Route.direct);
        return;
      } catch (e) {
        if (e is HandshakeException) rethrow;
        lanErr = e;
      }
    }
    if (haveLink || !c.relay) throw lanErr ?? 'no route';
    final relay = await ref.read(relayClientProvider.future);
    if (relay == null) throw 'relay is off';
    await openLink(() => relay.dial(id), Route.relay);
  }

  /// Dials, asks for a link and hands it to the link manager.
  Future<void> openLink(Future<PaltConn> Function() dial, String route) async {
    final conn = await dial();
    final reader = ByteReader(conn.input);
    try {
      writeOfferJson(conn, {
        'action': 'link',
        'transferId': randomHex(8),
        'files': const [],
        'totalSize': 0,
        'senderName': await ref.read(localNameProvider.future),
      });
      await conn.flush();
      final v = await reader.read(1).timeout(const Duration(seconds: 10));
      if (v == null || v[0] != 0x01) throw 'link refused';
    } catch (_) {
      conn.destroy();
      rethrow;
    }
    await ref.read(linkManagerProvider).run(conn, reader, route, initiator: true);
  }
}
