/// pipe.dart — the encrypted pipe between two paired devices, over the LAN or
/// a relayed stream.
///
/// Must stay byte-for-byte identical to desktop/internal/secure/pipe.go:
///
///   initiator → epkI | HMAC(authKey, "palt-hs-i|" session epkI)
///   responder → epkR | HMAC(authKey, "palt-hs-r|" session epkI epkR)
///   okm = HKDF-SHA256(X25519, salt=authKey, info="palt-relay-v1" epkI epkR, 64)
///   initiator→responder key = okm[:32], responder→initiator key = okm[32:]
///
/// Frames: u32 big-endian ciphertext length, then AES-256-GCM ciphertext+tag.
/// Nonce: 4 zero bytes + u64 big-endian frame counter per direction.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';

import '../palt_conn.dart';
import 'keys.dart';

const _maxFramePlain = 64 * 1024;
const _tagSize = 16;
const _flushEvery = 512 * 1024;

class HandshakeException implements Exception {
  @override
  String toString() =>
      'Secure handshake failed — are both devices paired with the same code?';
}

/// Runs the key exchange on [raw] (whose bytes are read through [reader])
/// and returns the encrypted connection.
Future<SecureConn> secureHandshake(
  Socket raw,
  ByteReader reader,
  GroupKeys keys,
  String session, {
  required bool initiator,
  String route = Route.relay,
}) async {
  final x = X25519();
  final pair = await x.newKeyPair();
  final epk = Uint8List.fromList((await pair.extractPublicKey()).bytes);
  final sess = utf8.encode(session);
  final hsI = utf8.encode('palt-hs-i|');
  final hsR = utf8.encode('palt-hs-r|');

  Uint8List epkI, epkR;
  const timeout = Duration(seconds: 15);
  if (initiator) {
    epkI = epk;
    raw.add([...epkI, ...hmacParts(keys.authKey, [hsI, sess, epkI])]);
    final msg = await reader.read(64).timeout(timeout);
    if (msg == null) throw HandshakeException();
    epkR = Uint8List.fromList(msg.sublist(0, 32));
    final want = hmacParts(keys.authKey, [hsR, sess, epkI, epkR]);
    if (!constantTimeEquals(msg.sublist(32), want)) throw HandshakeException();
  } else {
    final msg = await reader.read(64).timeout(timeout);
    if (msg == null) throw HandshakeException();
    epkI = Uint8List.fromList(msg.sublist(0, 32));
    final want = hmacParts(keys.authKey, [hsI, sess, epkI]);
    if (!constantTimeEquals(msg.sublist(32), want)) throw HandshakeException();
    epkR = epk;
    raw.add([...epkR, ...hmacParts(keys.authKey, [hsR, sess, epkI, epkR])]);
  }

  final shared = await x.sharedSecretKey(
    keyPair: pair,
    remotePublicKey: SimplePublicKey(initiator ? epkR : epkI, type: KeyPairType.x25519),
  );
  final okm = hkdfSha256(
    await shared.extractBytes(),
    keys.authKey,
    [...utf8.encode('palt-relay-v1'), ...epkI, ...epkR],
    64,
  );
  final i2r = SecretKey(okm.sublist(0, 32));
  final r2i = SecretKey(okm.sublist(32));
  return SecureConn._(
    raw,
    reader,
    route,
    writeKey: initiator ? i2r : r2i,
    readKey: initiator ? r2i : i2r,
  );
}

Uint8List _nonce(int n) {
  final b = ByteData(12);
  b.setUint32(4, (n >> 32) & 0xffffffff);
  b.setUint32(8, n & 0xffffffff);
  return b.buffer.asUint8List();
}

/// A [PaltConn] whose payload is framed and AES-GCM encrypted.
class SecureConn implements PaltConn {
  final Socket _raw;
  final ByteReader _reader;
  @override
  final String route;
  final SecretKey _writeKey;
  final SecretKey _readKey;
  final _aead = AesGcm.with256bits();
  int _wCount = 0;
  int _rCount = 0;
  Future<void> _writeChain = Future.value();
  int _unflushed = 0;
  Object? _writeError;

  SecureConn._(this._raw, this._reader, this.route,
      {required SecretKey writeKey, required SecretKey readKey})
      : _writeKey = writeKey,
        _readKey = readKey;

  @override
  String get remoteLabel => '$route ${_raw.remoteAddress.address}';

  /// The peer's address (the relay's, for relayed pipes).
  String get remoteAddress => _raw.remoteAddress.address;

  @override
  late final Stream<Uint8List> input = _decrypt();

  Stream<Uint8List> _decrypt() async* {
    while (true) {
      final hdr = await _reader.read(4);
      if (hdr == null) return; // clean end of stream
      final size = ByteData.sublistView(hdr).getUint32(0);
      if (size < _tagSize || size > _maxFramePlain + _tagSize) {
        throw const FormatException('Secure: bad frame size');
      }
      final ct = await _reader.read(size);
      if (ct == null) throw const SocketException('Secure: connection cut mid-frame');
      final box = SecretBox(
        Uint8List.sublistView(ct, 0, size - _tagSize),
        nonce: _nonce(_rCount),
        mac: Mac(Uint8List.sublistView(ct, size - _tagSize)),
      );
      List<int> plain;
      try {
        plain = await _aead.decrypt(box, secretKey: _readKey);
      } on SecretBoxAuthenticationError {
        throw const FormatException('Secure: frame failed authentication');
      }
      _rCount++;
      yield plain is Uint8List ? plain : Uint8List.fromList(plain);
    }
  }

  Future<void> _encryptAndSend(List<int> data) async {
    var offset = 0;
    while (offset < data.length) {
      final end = (offset + _maxFramePlain).clamp(0, data.length);
      final chunk = data is Uint8List
          ? Uint8List.sublistView(data, offset, end)
          : data.sublist(offset, end);
      final box = await _aead.encrypt(chunk, secretKey: _writeKey, nonce: _nonce(_wCount));
      _wCount++;
      final len = box.cipherText.length + _tagSize;
      final frame = Uint8List(4 + len);
      ByteData.sublistView(frame).setUint32(0, len);
      frame.setRange(4, 4 + box.cipherText.length, box.cipherText);
      frame.setRange(4 + box.cipherText.length, frame.length, box.mac.bytes);
      _raw.add(frame);
      offset = end;
    }
  }

  @override
  void add(List<int> data) {
    if (data.isEmpty) return;
    _writeChain = _writeChain.then((_) => _encryptAndSend(data)).catchError((Object e) {
      _writeError ??= e;
    });
  }

  @override
  Future<void> addStream(Stream<List<int>> stream) async {
    await for (final chunk in stream) {
      add(chunk);
      _unflushed += chunk.length;
      if (_unflushed >= _flushEvery) {
        // Backpressure: let the network drain before reading more.
        await flush();
      }
    }
  }

  @override
  Future<void> flush() async {
    await _writeChain;
    if (_writeError != null) throw _writeError!;
    _unflushed = 0;
    await _raw.flush();
  }

  @override
  Future<void> close() async {
    try {
      await flush();
    } catch (_) {}
    _raw.destroy();
  }

  @override
  void destroy() => _raw.destroy();
}
