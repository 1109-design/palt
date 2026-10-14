/// peer_provider.dart — Riverpod state layer for PALT peer discovery.
///
/// Architecture:
///   discoveryServiceProvider  → singleton DiscoveryService (auto-started)
///   localIpProvider           → AsyncProvider for the device's own IP
///   lanPeerListProvider       → StreamProvider (live LAN List<Peer>)
///   peerListProvider          → LAN + relay peers, each tagged with `via`
///   peerNotifierProvider      → AsyncNotifier with imperative rescan()
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/peer.dart';
import '../services/discovery_service.dart';
import '../services/relay/relay_client.dart';
import 'relay_provider.dart';

// ─── DiscoveryService singleton ────────────────────────────────────────────────

final discoveryServiceProvider = Provider<DiscoveryService>((ref) {
  final service = DiscoveryService();

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

/// Every reachable peer: LAN peers first (direct is always preferred), plus
/// relay peers that aren't on this network.
final peerListProvider = Provider<AsyncValue<List<Peer>>>((ref) {
  final lan = ref.watch(lanPeerListProvider);
  final relay = ref.watch(relayPeersProvider).valueOrNull ?? const <RelayPeer>[];

  final lanPeers = lan.valueOrNull ?? const <Peer>[];
  final onLan = {for (final p in lanPeers) p.deviceName};
  final merged = [
    ...lanPeers,
    for (final r in relay)
      if (!onLan.contains(r.name))
        Peer(
          id: 'relay-${r.id}',
          deviceName: r.name,
          ipAddress: 'relay',
          port: 0,
          os: r.os,
          via: 'relay',
        ),
  ];
  // Still scanning the LAN with nothing found anywhere: keep the loading state.
  if (merged.isEmpty && lan.isLoading) return lan;
  return AsyncData(merged);
});

// End of peer_provider.dart
