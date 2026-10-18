/// bluetooth.dart — each device's Bluetooth on the other: on or off, what's
/// connected (with battery), and switching it.
///
/// Android 13+ doesn't let apps switch Bluetooth: a computer turning this
/// phone's on gets the system prompt here ("asked"); turning it off opens
/// Bluetooth settings ("settings") (BluetoothInfo.kt).
///
/// Packet types (must match desktop/bluetooth_app.go). Requests carry an id
/// and are answered by "<type>.reply" with the same id.
///   bluetooth      either way  {available, on, devices: [{name, kind,
///                              battery}]}, on link up and on change
///   bluetooth.set  either way  {on} → {result: done|asked|settings, error?}
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../services/link/link.dart';

const _channel = MethodChannel('palt/bluetooth');

class BluetoothDeviceInfo {
  final String name, kind;
  final int battery; // -1 unknown
  const BluetoothDeviceInfo(this.name, this.kind, this.battery);
}

class BluetoothState {
  final bool available, on;
  final List<BluetoothDeviceInfo> devices;
  const BluetoothState({this.available = false, this.on = false, this.devices = const []});

  factory BluetoothState.fromJson(Map<String, dynamic> j) => BluetoothState(
        available: j['available'] as bool? ?? false,
        on: j['on'] as bool? ?? false,
        devices: [
          for (final d in (j['devices'] as List?) ?? const [])
            BluetoothDeviceInfo(
                (d as Map)['name'] as String? ?? '', d['kind'] as String? ?? '', (d['battery'] as num?)?.toInt() ?? -1),
        ],
      );
}

/// Re-sends this phone's Bluetooth (e.g. after the permission is granted).
void Function() refreshBluetooth = () {};

/// Linked devices' Bluetooth, by device name.
final peerBluetoothProvider = StateProvider<Map<String, BluetoothState>>((ref) => const {});

/// Asks a computer to turn its Bluetooth on or off.
Future<void> setPeerBluetooth(Link l, bool on) async {
  final p = await l.request('bluetooth.set', body: {'on': on});
  final err = p.body?['error'] as String?;
  if (err != null && err.isNotEmpty) throw err;
}

void setupBluetooth(ProviderContainer c) {
  final links = c.read(linkManagerProvider);

  links.handle('bluetooth', (l, p) {
    if (p.body == null) return;
    c.read(peerBluetoothProvider.notifier).update((m) => {...m, l.peer.name: BluetoothState.fromJson(p.body!)});
  });

  links.handle('bluetooth.set', (l, p) {
    unawaited(() async {
      Map<String, dynamic> reply;
      try {
        reply = {
          'result': await _channel.invokeMethod<String>('set', {'on': p.body?['on'] == true})
        };
      } on PlatformException catch (e) {
        reply = {'error': e.message ?? e.code};
      }
      await l.send('bluetooth.set.reply', id: p.id, body: reply).catchError((Object _) {});
    }());
  });

  // This phone's Bluetooth: to each device as it links up, and on change.
  final told = <Link>{};
  Future<void> send({bool all = false}) async {
    Map<String, dynamic> st;
    try {
      st = Map<String, dynamic>.from(await _channel.invokeMethod<Map>('state') ?? const {});
    } catch (_) {
      return;
    }
    told.removeWhere((l) => l.isClosed);
    for (final l in links.links) {
      if ((all || !told.contains(l)) && l.supports('bluetooth')) {
        told.add(l);
        unawaited(l.send('bluetooth', body: st).catchError((Object _) {}));
      }
    }
  }

  Timer? settle;
  _channel.setMethodCallHandler((call) async {
    if (call.method == 'changed') {
      // Connecting a headset fires several broadcasts; send once.
      settle?.cancel();
      settle = Timer(const Duration(milliseconds: 800), () => send(all: true));
    }
  });
  refreshBluetooth = () => send(all: true);
  links.changes.listen((_) => send());
  // Headset batteries drift without a broadcast every time.
  Timer.periodic(const Duration(minutes: 2), (_) => send(all: true));
  unawaited(send());
}
