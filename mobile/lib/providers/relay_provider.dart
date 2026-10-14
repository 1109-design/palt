/// relay_provider.dart — Riverpod wiring for the palt-relay connection.
///
///   relayCodeProvider    → saved pairing code (null = relay off)
///   relayClientProvider  → connected RelayClient for that code
///   relayStatusProvider  → live connection state for the UI
///   relayPeersProvider   → my other devices currently on the relay
library;

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/discovery_service.dart';
import '../services/relay/relay_client.dart';
import '../services/relay/relay_code.dart';
import '../services/transfer_service.dart';
import 'trust_provider.dart';

const _codeKey = 'palt_relay_code';
const _deviceIdKey = 'palt_relay_device_id';

final relayCodeProvider = StateNotifierProvider<RelayCodeNotifier, String?>((ref) {
  return RelayCodeNotifier(ref);
});

class RelayCodeNotifier extends StateNotifier<String?> {
  final Ref ref;
  RelayCodeNotifier(this.ref)
      : super(ref.read(sharedPreferencesProvider).getString(_codeKey));

  /// Validates and saves a pairing code; null or empty turns the relay off.
  /// Throws [FormatException] with a readable message for bad codes.
  Future<void> set(String? code) async {
    final prefs = ref.read(sharedPreferencesProvider);
    final trimmed = code?.trim() ?? '';
    if (trimmed.isEmpty) {
      await prefs.remove(_codeKey);
      state = null;
      return;
    }
    final normalized = RelayCode.parse(trimmed).toString();
    await prefs.setString(_codeKey, normalized);
    state = normalized;
  }
}

final relayClientProvider = FutureProvider<RelayClient?>((ref) async {
  final codeStr = ref.watch(relayCodeProvider);
  if (codeStr == null) return null;
  RelayCode code;
  try {
    code = RelayCode.parse(codeStr);
  } catch (_) {
    return null;
  }

  final prefs = ref.read(sharedPreferencesProvider);
  var deviceId = prefs.getString(_deviceIdKey);
  if (deviceId == null) {
    deviceId = randomHex(8);
    await prefs.setString(_deviceIdKey, deviceId);
  }

  final client = RelayClient(code, deviceId, await fetchLocalDeviceName(), 'android');
  client.onIncoming = (conn, from) {
    ref.read(transferServiceProvider).handleConnection(conn);
  };
  client.start();

  // Android may drop the socket while the app is in the background.
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
