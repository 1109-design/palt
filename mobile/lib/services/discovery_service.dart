/// discovery_service.dart
///
/// mDNS advertisement + peer browsing for PALT on Android.
///
/// Responsibilities:
///   1. Advertises this device as `_palt._tcp` via Android NSD.
///   2. Browses for other `_palt._tcp` instances.
///   3. Answers and sends UDP broadcast beacons on the PALT port, a fallback
///      for networks where mDNS doesn't get through.
///   4. Surfaces the peer list as a [Stream<List<Peer>>] consumed by Riverpod.
///
/// Dependencies (pubspec.yaml):
///   nsd: ^2.0.0
///   network_info_plus: ^6.0.0
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:nsd/nsd.dart';
import 'package:network_info_plus/network_info_plus.dart';
import 'package:device_info_plus/device_info_plus.dart';

import '../models/peer.dart';

/// mDNS service type — must match the Go discovery package.
const _kServiceType = '_palt._tcp';

/// TCP port advertised. Matches [paltPort] in Go app.go.
const kPaltPort = 9876;

const _kBeaconInterval = Duration(seconds: 30);
const _kBeaconTtl = Duration(seconds: 95);

/// Manages mDNS advertising and peer discovery for the Android client.
class DiscoveryService {
  /// This install's PALT device ID, advertised so peers can match LAN
  /// entries to paired devices.
  final String deviceId;

  DiscoveryService({required this.deviceId});

  // ── State ──────────────────────────────────────────────────────────────────
  Registration? _registration;
  Discovery? _discovery;
  RawDatagramSocket? _udp;
  Timer? _beaconTimer;

  final Map<String, Peer> _peers = {};
  // When each beacon-only peer (keyed by IP) was last heard.
  final Map<String, DateTime> _beaconSeen = {};
  final Set<String> _nsdIps = {};
  final Map<String, DateTime> _replied = {};
  final _controller = StreamController<List<Peer>>.broadcast();

  String _localIp = '';
  String _deviceName = 'palt-device';

  // ── Public API ─────────────────────────────────────────────────────────────

  /// Live stream of the discovered peer list.
  /// Emits a new snapshot on every add/remove.
  Stream<List<Peer>> get peers => _controller.stream;

  /// Synchronous snapshot of the current peer list.
  List<Peer> get currentPeers => List.unmodifiable(_peers.values);

  /// Starts advertising and browsing. Safe to call multiple times.
  Future<void> start() async {
    _localIp = await getLocalIpAddress();
    _deviceName = await fetchLocalDeviceName();
    await _advertise();
    await _browse();
    await _startBeacon();
  }

  /// Starts over after a network change: new address, fresh browse,
  /// beacon right away.
  Future<void> rescan() async {
    _localIp = await getLocalIpAddress();
    if (_discovery != null) {
      try {
        await stopDiscovery(_discovery!);
      } catch (_) {}
      _discovery = null;
    }
    _peers.clear();
    _nsdIps.clear();
    _beaconSeen.clear();
    _emit();
    await _browse();
    _query();
  }

  /// Stops advertising and browsing, closes the stream.
  Future<void> stop() async {
    _beaconTimer?.cancel();
    _udp?.close();
    _udp = null;
    if (_discovery != null) {
      await stopDiscovery(_discovery!);
      _discovery = null;
    }
    if (_registration != null) {
      await unregister(_registration!);
      _registration = null;
    }
    if (!_controller.isClosed) await _controller.close();
  }

  // ── Advertisement ──────────────────────────────────────────────────────────

  Future<void> _advertise() async {
    if (_registration != null) return;

    final name = _deviceName;

    final service = Service(
      name: name,
      type: _kServiceType,
      port: kPaltPort,
      txt: _encodeTxt({'os': 'android', 'app': 'palt', 'id': deviceId}),
    );

    try {
      _registration = await register(service);
      _log('Advertising as "$name" on port $kPaltPort');
    } catch (e) {
      _log('ERROR advertising: $e');
    }
  }

  // ── Discovery ──────────────────────────────────────────────────────────────

  Future<void> _browse() async {
    if (_discovery != null) return;

    try {
      _discovery = await startDiscovery(
        _kServiceType,
        autoResolve: true,
        ipLookupType: IpLookupType.v4,
      );

      _discovery!.addServiceListener((service, status) {
        if (status == ServiceStatus.found) {
          _onFound(service);
        } else if (status == ServiceStatus.lost) {
          _onLost(service);
        }
      });

      _log('Browsing for $_kServiceType...');
    } catch (e) {
      _log('ERROR browsing: $e');
    }
  }

  // ── Handlers ───────────────────────────────────────────────────────────────

  void _onFound(Service service) {
    final name = service.name ?? 'unknown';
    final addresses = service.addresses;

    if (addresses == null || addresses.isEmpty) {
      _log('Found "$name" but no address resolved — skipping.');
      return;
    }

    final ip = addresses.first.address;

    // Skip self by comparing IPs natively
    if (ip == _localIp || ip == '127.0.0.1') {
      _log('Ignoring self via IP match: $ip');
      return;
    }
    final port = service.port ?? kPaltPort;
    final txt = _decodeTxt(service.txt);
    final peerOS = txt['os'] ?? 'unknown';
    final id = txt['id'] ?? '';
    if (id.isNotEmpty && id == deviceId) return;

    _nsdIps.add(ip);
    _upsert(Peer(
      id: ip,
      deviceName: name,
      ipAddress: ip,
      port: port,
      os: peerOS,
      deviceId: id.isNotEmpty ? id : (_peers[ip]?.deviceId ?? ''),
    ));
  }

  void _upsert(Peer peer) {
    final old = _peers[peer.ipAddress];
    if (old != null &&
        old.deviceName == peer.deviceName &&
        old.port == peer.port &&
        old.os == peer.os &&
        old.deviceId == peer.deviceId) {
      return;
    }
    _peers[peer.ipAddress] = peer;
    _emit();
    _log('Peer found: $peer');
  }

  // ── UDP beacon ─────────────────────────────────────────────────────────────

  Future<void> _startBeacon() async {
    try {
      final udp = await RawDatagramSocket.bind(InternetAddress.anyIPv4, kPaltPort);
      udp.broadcastEnabled = true;
      udp.listen((event) {
        if (event != RawSocketEvent.read) return;
        final dg = udp.receive();
        if (dg != null) _onBeacon(dg);
      });
      _udp = udp;
      _query();
      _beaconTimer = Timer.periodic(_kBeaconInterval, (_) {
        _pruneBeacon();
        _query();
      });
    } catch (e) {
      _log('UDP beacon unavailable: $e');
    }
  }

  List<int> _beaconMsg({required bool query}) => utf8.encode(jsonEncode({
        'palt': 2,
        'id': deviceId,
        'name': _deviceName,
        'os': 'android',
        'port': kPaltPort,
        if (query) 'q': true,
      }));

  void _query() {
    final udp = _udp;
    if (udp == null) return;
    final msg = _beaconMsg(query: true);
    final targets = {'255.255.255.255'};
    // The subnet broadcast too: some routers only pass that one.
    final ip = InternetAddress.tryParse(_localIp);
    if (ip != null && ip.type == InternetAddressType.IPv4) {
      final b = ip.rawAddress;
      targets.add('${b[0]}.${b[1]}.${b[2]}.255');
    }
    for (final t in targets) {
      try {
        udp.send(msg, InternetAddress(t), kPaltPort);
      } catch (_) {}
    }
  }

  void _onBeacon(Datagram dg) {
    Map<String, dynamic> m;
    try {
      m = jsonDecode(utf8.decode(dg.data)) as Map<String, dynamic>;
    } catch (_) {
      return;
    }
    final id = m['id'] as String? ?? '';
    if ((m['palt'] as int? ?? 0) < 2 || id.isEmpty || id == deviceId) return;
    final ip = dg.address.address;
    var port = m['port'] as int? ?? kPaltPort;
    if (port <= 0 || port > 65535) port = kPaltPort;
    _beaconSeen[ip] = DateTime.now();
    _upsert(Peer(
      id: ip,
      deviceName: m['name'] as String? ?? 'unknown',
      ipAddress: ip,
      port: port,
      os: m['os'] as String? ?? 'unknown',
      deviceId: id,
    ));
    if (m['q'] == true) {
      final last = _replied[ip];
      if (last == null || DateTime.now().difference(last) > const Duration(seconds: 1)) {
        _replied[ip] = DateTime.now();
        _udp?.send(_beaconMsg(query: false), dg.address, dg.port);
      }
    }
  }

  /// Drops peers only the beacon knew about once they stop answering.
  void _pruneBeacon() {
    final cutoff = DateTime.now().subtract(_kBeaconTtl);
    final stale = [
      for (final e in _beaconSeen.entries)
        if (e.value.isBefore(cutoff) && !_nsdIps.contains(e.key)) e.key
    ];
    if (stale.isEmpty) return;
    for (final ip in stale) {
      _beaconSeen.remove(ip);
      _peers.remove(ip);
    }
    _emit();
  }

  void _onLost(Service service) {
    final name = service.name ?? '';
    int removedCount = 0;
    _peers.removeWhere((key, value) {
      if (value.deviceName == name) {
        _nsdIps.remove(key);
        // Still answering beacons: keep it.
        if (_beaconSeen.containsKey(key)) return false;
        removedCount++;
        return true;
      }
      return false;
    });
    if (removedCount > 0) {
      _emit();
      _log('Peer lost: $name');
    }
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  void _emit() {
    if (!_controller.isClosed) {
      _controller.add(List.unmodifiable(_peers.values));
    }
  }

  Map<String, Uint8List?> _encodeTxt(Map<String, String> map) =>
      map.map((k, v) => MapEntry(k, Uint8List.fromList(v.codeUnits)));

  Map<String, String> _decodeTxt(Map<String, Uint8List?>? txt) {
    if (txt == null) return {};
    return txt.map(
      (k, v) => MapEntry(k, v != null ? String.fromCharCodes(v) : ''),
    );
  }

  void _log(String msg) => print('[DiscoveryService] $msg'); // ignore: avoid_print
}

/// The name this device advertises over mDNS. Peers list us under this name,
/// so transfers must carry it as senderName for history to line up per device.
Future<String> fetchLocalDeviceName() async {
  try {
    if (Platform.isAndroid) {
      final info = await DeviceInfoPlugin().androidInfo;
      return info.model;
    } else if (Platform.isIOS) {
      final info = await DeviceInfoPlugin().iosInfo;
      return info.name;
    }
  } catch (_) {}
  return Platform.localHostname.isNotEmpty ? Platform.localHostname : 'palt-mobile';
}

/// Returns the device's own WiFi IP for display purposes.
Future<String> getLocalIpAddress() async {
  try {
    final info = NetworkInfo();
    return (await info.getWifiIP()) ?? 'Unknown';
  } catch (_) {
    return 'Unknown';
  }
}
