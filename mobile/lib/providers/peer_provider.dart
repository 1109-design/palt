/// peer_provider.dart — Riverpod state layer for PALT peer discovery.
///
/// Architecture:
///   discoveryServiceProvider  → singleton DiscoveryService (auto-started)
///   localIpProvider           → AsyncProvider for the device's own IP
///   lanPeerListProvider       → StreamProvider (live LAN List<Peer>)
///   peerListProvider          → linked (paired) devices, then LAN and relay
///                               peers, each tagged with `via`
///   peerNotifierProvider      → AsyncNotifier with imperative rescan()
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/peer.dart';
import '../services/discovery_service.dart';
import '../services/link/link.dart';
import '../services/relay/relay_client.dart';
import 'link_provider.dart';
import 'pairing_provider.dart';
import 'relay_provider.dart';

// ─── DiscoveryService singleton ────────────────────────────────────────────────

final discoveryServiceProvider = Provider<DiscoveryService>((ref) {
  final service = DiscoveryService(deviceId: ref.read(pairingProvider).deviceId);

  service.start().catchError((Object e) {
    print('[PeerProvider] Failed to start discovery: $e'); // ignore: avoid_print
  });

  ref.onDispose(service.stop);
  return service;
});

// ─── Local IP ──────────────────────────────────────────────────────────────────

final localIpProvider = FutureProvider<String>((ref) async {
  return getLocalIpAddress();
});

// ─── Live peer list ────────────────────────────────────────────────────────────

/// Peers on the local network. Emits whenever one joins or leaves.
final lanPeerListProvider = StreamProvider<List<Peer>>((ref) {
  return ref.watch(discoveryServiceProvider).peers;
});

/// Every reachable peer: linked (paired) devices first, then other LAN
/// peers, then relay peers without a link yet.
final peerListProvider = Provider<AsyncValue<List<Peer>>>((ref) {
  final lan = ref.watch(lanPeerListProvider);
  final relay = ref.watch(relayPeersProvider).valueOrNull ?? const <RelayPeer>[];
  final links = ref.watch(linksProvider).valueOrNull ?? const <Link>[];

  final lanPeers = lan.valueOrNull ?? const <Peer>[];
  final seenIds = <String>{};
  final seenNames = <String>{};
  final merged = <Peer>[];
  for (final l in links) {
    final ip = l.remoteIp;
    final lanPort = lanPeers.where((p) => p.ipAddress == ip).map((p) => p.port).firstOrNull;
    merged.add(Peer(
      id: 'dev-${l.peer.id}',
      deviceName: l.peer.name,
      ipAddress: ip ?? 'relay',
      port: ip == null ? 0 : (lanPort ?? kPaltPort),
      os: l.peer.os,
      via: l.route,
      deviceId: l.peer.id,
      paired: true,
    ));
    seenIds.add(l.peer.id);
    seenNames.add(l.peer.name);
  }
  for (final p in lanPeers) {
    if (seenIds.contains(p.deviceId) || (p.deviceId.isEmpty && seenNames.contains(p.deviceName))) {
      continue;
    }
    merged.add(p);
    seenNames.add(p.deviceName);
    if (p.deviceId.isNotEmpty) seenIds.add(p.deviceId);
  }
  for (final r in relay) {
    if (seenIds.contains(r.id) || seenNames.contains(r.name)) continue;
    merged.add(Peer(
      id: 'relay-${r.id}',
      deviceName: r.name,
      ipAddress: 'relay',
      port: 0,
      os: r.os,
      via: 'relay',
      deviceId: r.id,
    ));
  }
  // Still scanning the LAN with nothing found anywhere: keep the loading state.
  if (merged.isEmpty && lan.isLoading) return lan;
  return AsyncData(merged);
});

// End of peer_provider.dart
