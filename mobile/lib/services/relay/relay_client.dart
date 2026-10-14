/// relay_client.dart — keeps this device registered with the palt-relay
/// server and opens/accepts encrypted pipes to my other devices.
///
/// Mirrors desktop/internal/relay/client.go.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';

import '../palt_conn.dart';
import 'relay_code.dart';
import 'secure_pipe.dart';

const _protocolVersion = 1;
const _pingInterval = Duration(seconds: 20);
const _readTimeout = Duration(seconds: 60);
const _dialTimeout = Duration(seconds: 10);

/// Another of my devices connected to the relay.
@immutable
class RelayPeer {
  final String id;
  final String name;
  final String os;
  const RelayPeer(this.id, this.name, this.os);
}

enum RelayState { off, connecting, online, error }

@immutable
class RelayStatus {
  final RelayState state;
  final String detail;
  final int peers;
  const RelayStatus(this.state, {this.detail = '', this.peers = 0});
  static const off = RelayStatus(RelayState.off);
}

class RelayClient {
  final RelayCode code;
  final RelayKeys keys;
  final String deviceId;
  final String name;
  final String os;

  /// Receives each accepted, already-encrypted pipe and who it is from.
  void Function(PaltConn conn, RelayPeer from)? onIncoming;

  final _peers = <String, RelayPeer>{};
  final _peersCtl = StreamController<List<RelayPeer>>.broadcast();
  final _statusCtl = StreamController<RelayStatus>.broadcast();
  RelayStatus _status = const RelayStatus(RelayState.connecting);

  bool _stopped = false;
  Socket? _ctl;
  Completer<void>? _wake;

  RelayClient(this.code, this.deviceId, this.name, this.os) : keys = code.keys;

  Stream<List<RelayPeer>> get peerStream => _peersCtl.stream;
  Stream<RelayStatus> get statusStream => _statusCtl.stream;
  List<RelayPeer> get peers => List.unmodifiable(_peers.values);
  RelayStatus get status => _status;

  RelayPeer? peerByName(String name) {
    for (final p in _peers.values) {
      if (p.name == name) return p;
    }
    return null;
  }

  void start() => _loop();

  void stop() {
    _stopped = true;
    _ctl?.destroy();
    _wake?.complete();
    _setPeers(const []);
    _setStatus(RelayStatus.off);
    _peersCtl.close();
    _statusCtl.close();
  }

  /// Skips the backoff wait, e.g. when the app returns to the foreground.
  void reconnectNow() {
    if (_wake != null && !_wake!.isCompleted) _wake!.complete();
  }

  void _setStatus(RelayStatus s) {
    _status = s;
    if (!_statusCtl.isClosed) _statusCtl.add(s);
  }

  void _setPeers(List<dynamic> list) {
    _peers.clear();
    for (final raw in list) {
      if (raw is! Map) continue;
      final id = raw['id'] as String? ?? '';
      final pname = raw['name'] as String? ?? '';
      final pos = raw['os'] as String? ?? '';
      final sig = raw['sig'] as String? ?? '';
      if (id == deviceId) continue;
      if (!keys.verifyPresence(id, pname, pos, sig)) {
        _log('Ignoring peer with bad signature: $pname');
        continue;
      }
      _peers[id] = RelayPeer(id, pname, pos);
    }
    if (!_peersCtl.isClosed) _peersCtl.add(peers);
    if (_status.state == RelayState.online) {
      _setStatus(RelayStatus(RelayState.online, detail: code.address, peers: _peers.length));
    }
  }

  Future<void> _loop() async {
    var backoff = const Duration(seconds: 2);
    while (!_stopped) {
      _setStatus(RelayStatus(RelayState.connecting, detail: code.address));
      final started = DateTime.now();
      String? error;
      try {
        await _session();
      } catch (e) {
        error = e.toString();
      }
      _setPeers(const []);
      if (_stopped) return;
      _log('Disconnected: ${error ?? 'closed'}');
      _setStatus(RelayStatus(RelayState.error, detail: error ?? 'Lost connection to relay'));
      if (DateTime.now().difference(started) > const Duration(minutes: 1)) {
        backoff = const Duration(seconds: 2);
      }
      _wake = Completer<void>();
      await Future.any([Future.delayed(backoff), _wake!.future]);
      _wake = null;
      if (backoff < const Duration(seconds: 30)) backoff *= 2;
    }
  }

  Future<void> _session() async {
    Socket sock;
    try {
      sock = await Socket.connect(code.host, code.port, timeout: _dialTimeout);
    } catch (_) {
      throw 'Cannot reach relay ${code.address}';
    }
    _ctl = sock;
    if (_stopped) {
      sock.destroy();
      return;
    }
    sock.setOption(SocketOption.tcpNoDelay, true);
    final reader = ByteReader(sock);
    void write(Map<String, dynamic> m) => sock.add(utf8.encode('${jsonEncode(m)}\n'));

    Timer? ping;
    try {
      write({
        't': 'register',
        'v': _protocolVersion,
        'group': keys.groupId,
        'id': deviceId,
        'name': name,
        'os': os,
        'sig': keys.presenceSig(deviceId, name, os),
      });
      final first = await _readMsg(reader);
      if (first == null) throw 'Relay did not answer';
      if (first['t'] != 'ok') throw 'Relay refused: ${first['msg']}';
      _setStatus(RelayStatus(RelayState.online, detail: code.address));
      _log('Online at ${code.address}');

      ping = Timer.periodic(_pingInterval, (_) {
        try {
          write({'t': 'ping'});
        } catch (_) {
          sock.destroy();
        }
      });

      while (!_stopped) {
        final m = await _readMsg(reader);
        if (m == null) {
          if (_stopped) return;
          throw 'Lost connection to relay';
        }
        switch (m['t']) {
          case 'peers':
            _setPeers((m['peers'] as List?) ?? const []);
          case 'incoming':
            unawaited(_accept(m['session'] as String? ?? '', m['from'] as String? ?? ''));
        }
      }
    } finally {
      ping?.cancel();
      sock.destroy();
    }
  }

  Future<Map<String, dynamic>?> _readMsg(ByteReader reader) async {
    final line = await reader.readLine().timeout(_readTimeout);
    if (line == null) return null;
    return jsonDecode(utf8.decode(line)) as Map<String, dynamic>;
  }

  Future<void> _accept(String session, String from) async {
    final peer = _peers[from];
    if (peer == null) {
      _log('Incoming session from unknown device $from; ignoring');
      return;
    }
    try {
      final conn = await _openPipe({
        't': 'accept',
        'v': _protocolVersion,
        'group': keys.groupId,
        'id': deviceId,
        'session': session,
      }, session, initiator: false);
      final handler = onIncoming;
      if (handler == null) {
        conn.destroy();
      } else {
        handler(conn, peer);
      }
    } catch (e) {
      _log('Accept from ${peer.name} failed: $e');
    }
  }

  /// Opens an encrypted pipe to [peerId] through the relay.
  Future<PaltConn> dial(String peerId) {
    final session = randomHex(16);
    return _openPipe({
      't': 'connect',
      'v': _protocolVersion,
      'group': keys.groupId,
      'id': deviceId,
      'to': peerId,
      'session': session,
    }, session, initiator: true);
  }

  Future<SecureConn> _openPipe(Map<String, dynamic> req, String session,
      {required bool initiator}) async {
    Socket sock;
    try {
      sock = await Socket.connect(code.host, code.port, timeout: _dialTimeout);
    } catch (_) {
      throw 'Cannot reach relay ${code.address}';
    }
    try {
      sock.setOption(SocketOption.tcpNoDelay, true);
      final reader = ByteReader(sock);
      sock.add(utf8.encode('${jsonEncode(req)}\n'));
      final line = await reader.readLine(max: 4096).timeout(const Duration(seconds: 25));
      if (line == null) throw 'Relay did not answer';
      final reply = jsonDecode(utf8.decode(line)) as Map<String, dynamic>;
      if (reply['t'] != 'ok') throw 'Relay: ${reply['msg']}';
      return await secureHandshake(sock, reader, keys, session, initiator: initiator);
    } catch (e) {
      sock.destroy();
      rethrow;
    }
  }

  void _log(String msg) => debugPrint('[RelayClient] $msg');
}
