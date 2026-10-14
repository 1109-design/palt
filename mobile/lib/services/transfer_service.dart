import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';


import '../models/history_entry.dart';
import '../providers/auto_copy_provider.dart';
import '../providers/trust_provider.dart';
import '../providers/history_provider.dart';
import '../models/peer.dart';
import 'discovery_service.dart';

/// Must match MaxOfferSize / MaxTextBytes in desktop/internal/transfer/protocol.go
const kMaxOfferBytes = 64 * 1024;
const kMaxTextBytes = 16 * 1024;

final transferServiceProvider = Provider<TransferService>((ref) {
  final service = TransferService(ref);
  service.startServer();
  ref.onDispose(service.stopServer);
  return service;
});

// ── State Models ─────────────────────────────────────────────────────────────

class FileMeta {
  final String name;
  final int size;

  FileMeta({required this.name, required this.size});

  factory FileMeta.fromJson(Map<String, dynamic> json) => FileMeta(
        name: json['name'],
        size: json['size'],
      );

  Map<String, dynamic> toJson() => {
        'name': name,
        'size': size,
      };
}

class OfferData {
  /// 'offer' for a file batch, 'text' for a message carried in [text].
  final String action;
  final String transferId;
  final List<FileMeta> files;
  final int totalSize;
  final String senderName;
  final String? text;

  OfferData({
    this.action = 'offer',
    required this.transferId,
    required this.files,
    required this.totalSize,
    required this.senderName,
    this.text,
  });

  bool get isText => action == 'text';

  factory OfferData.fromJson(Map<String, dynamic> json) => OfferData(
        action: json['action'] ?? 'offer',
        transferId: json['transferId'],
        files: ((json['files'] as List?) ?? [])
            .map((e) => FileMeta.fromJson(e))
            .toList(),
        totalSize: json['totalSize'] ?? 0,
        senderName: json['senderName'],
        text: json['text'],
      );

  Map<String, dynamic> toJson() => {
        'action': action,
        'transferId': transferId,
        'files': files.map((e) => e.toJson()).toList(),
        'totalSize': totalSize,
        'senderName': senderName,
        if (text != null) 'text': text,
      };
}

/// A text message that just arrived, for the UI to surface as a toast.
class IncomingText {
  final String senderName;
  final String text;
  final bool copied;
  const IncomingText(this.senderName, this.text, {this.copied = false});
}

enum TransferStatus { waiting, transferring, completed, error }

class TransferProgress {
  final String transferId;
  final int written;
  final int total;
  final String? currentFileName;
  final double? speed; // Bytes per second
  final int? sentItems;
  final int? totalItems;
  final TransferStatus status;
  final String? error;
  final String? filePath;

  const TransferProgress(
    this.transferId,
    this.written,
    this.total, {
    this.currentFileName,
    this.speed,
    this.sentItems,
    this.totalItems,
    this.status = TransferStatus.transferring,
    this.error,
    this.filePath,
  });
}

// ── Riverpod Providers for UI ────────────────────────────────────────────────

class ActiveOfferNotifier extends StateNotifier<OfferData?> {
  ActiveOfferNotifier() : super(null);
  
  Completer<bool>? _resolution;

  Future<bool> presentOffer(OfferData offer) {
    _resolution = Completer<bool>();
    state = offer;
    return _resolution!.future;
  }

  void accept() {
    _resolution?.complete(true);
    state = null;
  }

  void reject() {
    _resolution?.complete(false);
    state = null;
  }
}

final activeOfferProvider = StateNotifierProvider<ActiveOfferNotifier, OfferData?>((ref) => ActiveOfferNotifier());

final transferProgressProvider = StateProvider<TransferProgress?>((ref) => null);

final incomingTextProvider = StateProvider<IncomingText?>((ref) => null);

// ── Core Service ─────────────────────────────────────────────────────────────

class TransferService {
  final Ref ref;
  ServerSocket? _serverSocket;
  final _uuid = const Uuid();
  final Future<String> _localName = fetchLocalDeviceName();

  TransferService(this.ref);

  Future<void> startServer() async {
    try {
      _serverSocket = await ServerSocket.bind(InternetAddress.anyIPv4, 9876);
      print('[TransferService] Server listening on port 9876');
      _serverSocket!.listen(_handleIncomingConnection);
    } catch (e) {
      print('[TransferService] Error starting server: $e');
    }
  }

  void stopServer() {
    _serverSocket?.close();
  }

  // Parses length-prefixed JSON handshake and streams multiple files sequentially
  Future<void> _handleIncomingConnection(Socket socket) async {
    print('[TransferService] Got connection from ${socket.remoteAddress.address}');
    
    final iterator = StreamIterator(socket);
    BytesBuilder buffer = BytesBuilder();
    int expectedLength = -1;
    OfferData? offer;
    bool handshakeComplete = false;

    try {
      // 1. Handshake Phase
      while (await iterator.moveNext()) {
        final data = iterator.current;
        buffer.add(data);
        final bytes = buffer.toBytes();

        if (expectedLength == -1 && bytes.length >= 4) {
          expectedLength = ByteData.view(bytes.buffer).getUint32(0, Endian.big);
          if (expectedLength > kMaxOfferBytes) {
            socket.destroy();
            return;
          }
        }

        if (expectedLength != -1 && bytes.length >= 4 + expectedLength) {
          // We have the full JSON
          final jsonBytes = bytes.sublist(4, 4 + expectedLength);
          final jsonStr = utf8.decode(jsonBytes);
          offer = OfferData.fromJson(jsonDecode(jsonStr));

          // Text messages ride inside the handshake; ack and we're done.
          if (offer.isText) {
            await _receiveText(socket, offer);
            return;
          }

          // Surface to UI
          final isTrusted = ref.read(trustProvider.notifier).isTrusted(offer.senderName);
          bool accepted = false;
          
          if (isTrusted) {
            print('[TransferService] Auto-accepting from trusted sender: ${offer.senderName}');
            accepted = true;
          } else {
            accepted = await ref.read(activeOfferProvider.notifier).presentOffer(offer);
          }

          if (!accepted) {
            socket.add([0x00]); // Reject
            socket.destroy();
            return;
          }

          // Accepted
          socket.add([0x01]);

          // Shift overflow bytes left over from the handshake segment
          buffer.clear();
          if (bytes.length > 4 + expectedLength) {
            buffer.add(bytes.sublist(4 + expectedLength));
          }

          handshakeComplete = true;
          break; // Exit to data phase
        }
      }

      if (!handshakeComplete || offer == null) {
        socket.destroy();
        return;
      }

      // 2. Setup download directory
      final publicDir = Directory('/storage/emulated/0/Download/PALT');
      if (!await publicDir.exists()) {
        try {
          await publicDir.create(recursive: true);
        } catch (e) {
          print('[TransferService] Could not create PALT directory: $e');
        }
      }
      
      final dirPath = publicDir.path;
      final stopwatch = Stopwatch()..start();

      // 3. Sequential Multi-file Pipelining
      int totalWritten = 0;
      final totalFiles = offer.files.length;
      List<HistoryFile> hFiles = [];
      for (var f in offer.files) {
        hFiles.add(HistoryFile(name: f.name, size: f.size));
      }

      for (int i = 0; i < totalFiles; i++) {
        final fileMeta = offer.files[i];
        final savePath = '$dirPath/${fileMeta.name}';
        final fileSink = File(savePath).openWrite();
        int fileWritten = 0;

        // Drain overflow buffer first
        if (buffer.isNotEmpty) {
          final bytesStr = buffer.toBytes();
          final available = bytesStr.length;
          final needed = fileMeta.size - fileWritten;

          if (available <= needed) {
            fileSink.add(bytesStr);
            fileWritten += available;
            totalWritten += available;
            buffer.clear();
          } else {
            fileSink.add(bytesStr.sublist(0, needed));
            fileWritten += needed;
            totalWritten += needed;
            buffer.clear();
            buffer.add(bytesStr.sublist(needed));
          }
        }

        // Stream exactly remaining raw files bytes directly from socket iterator
        while (fileWritten < fileMeta.size && await iterator.moveNext()) {
          final data = iterator.current;
          int needed = fileMeta.size - fileWritten;

          if (data.length <= needed) {
            fileSink.add(data);
            fileWritten += data.length;
            totalWritten += data.length;
          } else {
            fileSink.add(data.sublist(0, needed));
            fileWritten += needed;
            totalWritten += needed;
            
            // Re-buffer the overflow bytes for the next file loop!
            buffer.add(data.sublist(needed));
          }

          // Report progress 
          if (totalWritten % (1024 * 512) < data.length || fileWritten == fileMeta.size) {
            final elapsedSec = stopwatch.elapsedMilliseconds / 1000.0;
            final speed = elapsedSec > 0 ? totalWritten / elapsedSec : 0.0;
            
            ref.read(transferProgressProvider.notifier).state = 
                TransferProgress(offer.transferId, fileWritten, fileMeta.size, 
                                 currentFileName: fileMeta.name,
                                 speed: speed,
                                 sentItems: i+1, totalItems: totalFiles);
          }
        }

        await fileSink.flush();
        await fileSink.close();
        print('[TransferService] Downloaded File ${i+1}/${totalFiles}: ${fileMeta.name}');
      }

      print('[TransferService] Batch Download complete!');
      socket.destroy();
      stopwatch.stop();

      for (var hFile in hFiles) {
        final entry = HistoryEntry(
          id: '${offer.transferId}_${hFile.name}',
          partnerName: offer.senderName,
          files: [hFile],
          totalSize: hFile.size,
          direction: 'incoming',
          timestamp: DateTime.now(),
          status: 'completed',
          durationMillis: stopwatch.elapsedMilliseconds,
        );
        ref.read(historyProvider.notifier).addEntry(entry);
      }
      
      ref.read(transferProgressProvider.notifier).state = 
          TransferProgress(offer.transferId, totalWritten, offer.totalSize, 
                           sentItems: totalFiles, totalItems: totalFiles,
                           status: TransferStatus.completed, filePath: dirPath);

      Future.delayed(const Duration(seconds: 5), () {
        final current = ref.read(transferProgressProvider);
        if (current?.status == TransferStatus.completed && current?.transferId == offer!.transferId) {
          ref.read(transferProgressProvider.notifier).state = null;
        }
      });

    } catch (e) {
      print('[TransferService] Stream processing error: $e');
      
      if (offer != null) {
        for (var f in offer.files) {
          final entry = HistoryEntry(
            id: '${offer.transferId}_${f.name}',
            partnerName: offer.senderName,
            files: [HistoryFile(name: f.name, size: f.size)],
            totalSize: f.size,
            direction: 'incoming',
            timestamp: DateTime.now(),
            status: 'error',
            errorMessage: e.toString(),
            durationMillis: 0,
          );
          ref.read(historyProvider.notifier).addEntry(entry);
        }
      }

      ref.read(transferProgressProvider.notifier).state = 
          TransferProgress(offer?.transferId ?? 'unknown', 0, offer?.totalSize ?? 0, 
                           status: TransferStatus.error, error: e.toString());
      socket.destroy();
    }
  }

  Future<void> _receiveText(Socket socket, OfferData offer) async {
    final text = offer.text ?? '';
    final ok = text.isNotEmpty && utf8.encode(text).length <= kMaxTextBytes;
    socket.add([ok ? 0x01 : 0x00]);
    await socket.flush();
    socket.destroy();
    if (!ok) return;

    // Auto-copy is opt-in per device and only for trusted devices, so an
    // arbitrary LAN peer can't overwrite the clipboard.
    var copied = false;
    if (ref.read(trustProvider.notifier).isTrusted(offer.senderName) &&
        ref.read(autoCopyProvider.notifier).isEnabled(offer.senderName)) {
      try {
        await Clipboard.setData(ClipboardData(text: text));
        copied = true;
      } catch (e) {
        print('[TransferService] Auto-copy failed: $e');
      }
    }

    ref.read(historyProvider.notifier).addEntry(HistoryEntry(
          id: offer.transferId,
          partnerName: offer.senderName,
          files: const [],
          totalSize: utf8.encode(text).length,
          direction: 'incoming',
          timestamp: DateTime.now(),
          status: 'completed',
          durationMillis: 0,
          kind: 'text',
          text: text,
          copied: copied,
        ));
    ref.read(incomingTextProvider.notifier).state =
        IncomingText(offer.senderName, text, copied: copied);
  }

  // ── Send Text ───────────────────────────────────────────────────────────────

  /// Sends a text message and records it in history. Throws with a
  /// user-readable message when the peer can't take it.
  Future<void> sendText(Peer peer, String text) async {
    final transferId = _uuid.v4();
    final bytes = utf8.encode(text).length;
    String? error;

    try {
      if (text.trim().isEmpty) throw 'Message is empty';
      if (bytes > kMaxTextBytes) {
        throw 'Message is too long (max ${kMaxTextBytes ~/ 1024} KB)';
      }

      final socket = await Socket.connect(peer.ipAddress, peer.port,
          timeout: const Duration(seconds: 5));
      try {
        final offer = OfferData(
          action: 'text',
          transferId: transferId,
          files: const [],
          totalSize: 0,
          senderName: await _localName,
          text: text,
        );
        final jsonBytes = utf8.encode(jsonEncode(offer.toJson()));
        final lengthBuffer = ByteData(4)
          ..setUint32(0, jsonBytes.length, Endian.big);
        socket.add(lengthBuffer.buffer.asUint8List());
        socket.add(jsonBytes);
        await socket.flush();

        final verdict = Completer<int>();
        socket.listen(
          (data) {
            if (!verdict.isCompleted && data.isNotEmpty) verdict.complete(data[0]);
          },
          onError: (e) {
            if (!verdict.isCompleted) verdict.completeError(e);
          },
          // Builds without text support drop the connection without a verdict.
          onDone: () {
            if (!verdict.isCompleted) {
              verdict.completeError(
                  'This device does not support text messages — update PALT on it');
            }
          },
          cancelOnError: true,
        );

        final v = await verdict.future.timeout(const Duration(seconds: 10),
            onTimeout: () => throw 'No reply from ${peer.deviceName}');
        if (v != 0x01) throw 'Message rejected by ${peer.deviceName}';
      } finally {
        socket.destroy();
      }
    } on SocketException catch (e) {
      error = 'Could not reach ${peer.deviceName}: ${e.osError?.message ?? e.message}';
    } catch (e) {
      error = e.toString();
    }

    ref.read(historyProvider.notifier).addEntry(HistoryEntry(
          id: transferId,
          partnerName: peer.deviceName,
          files: const [],
          totalSize: bytes,
          direction: 'outgoing',
          timestamp: DateTime.now(),
          status: error == null ? 'completed' : 'error',
          errorMessage: error,
          durationMillis: 0,
          kind: 'text',
          text: text,
        ));

    if (error != null) throw error;
  }

  // ── Send Files ──────────────────────────────────────────────────────────────

  Future<void> sendFiles(Peer peer, List<File> files) async {
    int totalSize = 0;
    List<FileMeta> metaFiles = [];

    for (var file in files) {
      final size = await file.length();
      totalSize += size;
      metaFiles.add(FileMeta(name: file.uri.pathSegments.last, size: size));
    }

    if (metaFiles.isEmpty) return;

    final transferId = _uuid.v4();
    final senderName = await _localName;
    final totalFiles = metaFiles.length;
    final stopwatch = Stopwatch();

    ref.read(transferProgressProvider.notifier).state = 
        TransferProgress(transferId, 0, totalSize, sentItems: 0, totalItems: totalFiles);

    List<HistoryFile> hFiles = metaFiles.map((m) => HistoryFile(name: m.name, size: m.size)).toList();

    try {
      final socket = await Socket.connect(peer.ipAddress, peer.port, timeout: const Duration(seconds: 5));
      print('[TransferClient] Connected to ${peer.ipAddress}:${peer.port}');
      stopwatch.start();

      final offer = OfferData(
        transferId: transferId,
        files: metaFiles,
        totalSize: totalSize,
        senderName: senderName,
      );

      final jsonBytes = utf8.encode(jsonEncode(offer.toJson()));
      final lengthBuffer = ByteData(4)..setUint32(0, jsonBytes.length, Endian.big);
      socket.add(lengthBuffer.buffer.asUint8List());
      socket.add(jsonBytes);

      ref.read(transferProgressProvider.notifier).state = TransferProgress(
        transferId,
        0,
        totalSize,
        sentItems: 0,
        totalItems: totalFiles,
        status: TransferStatus.waiting,
      );

      // Wait for 1-byte verdict
      final completer = Completer<bool>();
      socket.listen(
        (data) {
          if (!completer.isCompleted && data.isNotEmpty) {
            completer.complete(data[0] == 0x01);
          }
        },
        onError: (err) {
          if (!completer.isCompleted) completer.completeError(err);
        },
      );

      final accepted = await completer.future.timeout(const Duration(minutes: 1));
      if (!accepted) {
        ref.read(transferProgressProvider.notifier).state = TransferProgress(
          transferId,
          0,
          totalSize,
          status: TransferStatus.error,
          error: 'Transfer rejected by peer',
        );
        return;
      }

      print('[TransferClient] Offer accepted. Sending $totalFiles files...');

      int totalWritten = 0;

      for (int i = 0; i < files.length; i++) {
        final stream = files[i].openRead();
        final fileSize = metaFiles[i].size;
        int fileWritten = 0;

        final mappedStream = stream.map((chunk) {
          fileWritten += chunk.length;
          totalWritten += chunk.length;

          if (totalWritten % (1024 * 512) < chunk.length || fileWritten == fileSize) {
              final elapsedSec = stopwatch.elapsedMilliseconds / 1000.0;
              final speed = elapsedSec > 0 ? totalWritten / elapsedSec : 0.0;

              ref.read(transferProgressProvider.notifier).state = 
                  TransferProgress(transferId, fileWritten, fileSize, 
                                   currentFileName: metaFiles[i].name,
                                   speed: speed,
                                   sentItems: i+1, totalItems: totalFiles);
          }
          return chunk;
        });

        await socket.addStream(mappedStream);

      }

      await socket.flush();
      socket.destroy();
      stopwatch.stop();

      print('[TransferClient] Transfer complete!');

      for (var hFile in hFiles) {
        final entry = HistoryEntry(
          id: '${transferId}_${hFile.name}',
          partnerName: peer.deviceName,
          files: [hFile],
          totalSize: hFile.size,
          direction: 'outgoing',
          timestamp: DateTime.now(),
          status: 'completed',
          durationMillis: stopwatch.elapsedMilliseconds,
        );
        ref.read(historyProvider.notifier).addEntry(entry);
      }
      
      ref.read(transferProgressProvider.notifier).state = 
          TransferProgress(transferId, totalSize, totalSize, 
                           sentItems: totalFiles, totalItems: totalFiles, 
                           status: TransferStatus.completed);
          
      Future.delayed(const Duration(seconds: 5), () {
        final current = ref.read(transferProgressProvider);
        if (current?.status == TransferStatus.completed && current?.transferId == transferId) {
          ref.read(transferProgressProvider.notifier).state = null;
        }
      });

    } catch (e) {
      print('[TransferClient] Error during send: $e');
      stopwatch.stop();

      for (var hFile in hFiles) {
        final entry = HistoryEntry(
          id: '${transferId}_${hFile.name}',
          partnerName: peer.deviceName,
          files: [hFile],
          totalSize: hFile.size,
          direction: 'outgoing',
          timestamp: DateTime.now(),
          status: 'error',
          errorMessage: e.toString(),
          durationMillis: stopwatch.elapsedMilliseconds,
        );
        ref.read(historyProvider.notifier).addEntry(entry);
      }

      ref.read(transferProgressProvider.notifier).state = 
                TransferProgress(transferId, 0, totalSize, status: TransferStatus.error, error: e.toString());
    }
  }
}
