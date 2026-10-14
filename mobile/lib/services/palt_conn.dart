/// palt_conn.dart — one transport for the PALT protocol.
///
/// The transfer code talks to a [PaltConn] so the same send/receive logic
/// runs over a direct LAN [Socket] or an encrypted relay pipe.
library;

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

/// How a connection reaches the other device.
class Route {
  static const direct = 'direct';
  static const relay = 'relay';
}

abstract class PaltConn {
  /// Incoming bytes. Single subscription.
  Stream<Uint8List> get input;

  /// Queues bytes for sending.
  void add(List<int> data);

  /// Sends a whole stream, waiting as the network drains.
  Future<void> addStream(Stream<List<int>> stream);

  /// Resolves once queued bytes have been handed to the OS.
  Future<void> flush();

  /// Flushes, then closes the connection.
  Future<void> close();

  /// Closes immediately, dropping anything unsent.
  void destroy();

  /// 'direct' or 'relay'.
  String get route;

  /// For logs.
  String get remoteLabel;
}

/// A direct LAN connection.
class SocketConn implements PaltConn {
  final Socket socket;
  SocketConn(this.socket);

  @override
  Stream<Uint8List> get input => socket;
  @override
  void add(List<int> data) => socket.add(data);
  @override
  Future<void> addStream(Stream<List<int>> stream) => socket.addStream(stream);
  @override
  Future<void> flush() => socket.flush();
  @override
  Future<void> close() async {
    try {
      await socket.flush();
    } catch (_) {}
    socket.destroy();
  }

  @override
  void destroy() => socket.destroy();
  @override
  String get route => Route.direct;
  @override
  String get remoteLabel => socket.remoteAddress.address;
}

/// Reads exact byte counts and lines from a byte stream, keeping leftovers.
class ByteReader {
  final StreamIterator<Uint8List> _it;
  final BytesBuilder _buf = BytesBuilder(copy: false);
  Uint8List _pending = Uint8List(0);

  ByteReader(Stream<Uint8List> stream) : _it = StreamIterator(stream);

  /// Returns exactly [n] bytes, or null if the stream ended first.
  Future<Uint8List?> read(int n) async {
    while (_pending.length < n) {
      if (!await _it.moveNext()) return null;
      _buf.add(_pending);
      _buf.add(_it.current);
      _pending = _buf.takeBytes();
    }
    final out = Uint8List.sublistView(_pending, 0, n);
    _pending = Uint8List.sublistView(_pending, n);
    return out;
  }

  /// Returns the next '\n'-terminated line without the newline, or null at
  /// end of stream. Throws if a line exceeds [max] bytes.
  Future<Uint8List?> readLine({int max = 64 * 1024}) async {
    var scanFrom = 0;
    while (true) {
      final i = _pending.indexOf(10, scanFrom);
      if (i >= 0) {
        final line = Uint8List.sublistView(_pending, 0, i);
        _pending = Uint8List.sublistView(_pending, i + 1);
        return line;
      }
      if (_pending.length > max) throw const FormatException('line too long');
      scanFrom = _pending.length;
      if (!await _it.moveNext()) return null;
      _buf.add(_pending);
      _buf.add(_it.current);
      _pending = _buf.takeBytes();
    }
  }

  /// Bytes already received but not yet consumed.
  Uint8List takeLeftover() {
    final out = _pending;
    _pending = Uint8List(0);
    return out;
  }

  /// Remaining chunks after whatever was read so far.
  Stream<Uint8List> rest() async* {
    final left = takeLeftover();
    if (left.isNotEmpty) yield left;
    while (await _it.moveNext()) {
      yield _it.current;
    }
  }

  Future<void> cancel() => _it.cancel();
}
