/// relay_provider.dart — Riverpod wiring for the palt-relay connection.
///
///   relayClientProvider  → connected RelayClient for the pairing's relay
///   relayStatusProvider  → live connection state for the UI
///   relayPeersProvider   → my other devices currently on the relay
library;

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/discovery_service.dart';
import '../services/relay/relay_client.dart';
import '../services/transfer_service.dart';
import 'pairing_provider.dart';

final relayClientProvider = FutureProvider<RelayClient?>((ref) async {
  // Restart only when the relay, key or ID change.
  final codeStr = ref.watch(pairingProvider.select((p) => p.relayCode?.toString()));
  final deviceId = ref.watch(pairingProvider.select((p) => p.deviceId));
  final code = ref.read(pairingProvider).relayCode;
  if (codeStr == null || code == null) return null;

  final client = RelayClient(code, deviceId, await fetchLocalDeviceName(), 'android');
  client.onIncoming = (conn, from) {
    ref.read(transferServiceProvider).handlePaired(conn, from.id);
  };
  client.start();

  // Android may drop the socket while the app is asleep.
  final lifecycle = AppLifecycleListener(onResume: client.reconnectNow);
  ref.onDispose(() {
    lifecycle.dispose();
    client.stop();
  });
  return client;
});

final relayStatusProvider = StreamProvider<RelayStatus>((ref) async* {
  final client = await ref.watch(relayClientProvider.future);
  if (client == null) {
    yield RelayStatus.off;
    return;
  }
  yield client.status;
  yield* client.statusStream;
});

final relayPeersProvider = StreamProvider<List<RelayPeer>>((ref) async* {
  final client = await ref.watch(relayClientProvider.future);
  if (client == null) {
    yield const [];
    return;
  }
  yield client.peers;
  yield* client.peerStream;
});
