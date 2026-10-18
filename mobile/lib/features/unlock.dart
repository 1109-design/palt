/// unlock.dart — the phone's fingerprint unlocks paired computers (lock
/// screen, admin prompts, sudo).
///
/// The computer makes a one-time challenge; the phone shows the fingerprint
/// prompt and signs it with a key kept in the Android Keystore that only
/// works right after a fingerprint (Unlock.kt). One key per computer.
///
/// Packet types (must match desktop/unlock_app.go). Requests carry an id and
/// are answered by "<type>.reply" with the same id.
///   unlock.setup   desktop → phone  {} → {publicKey} (base64 X.509, P-256)
///   unlock.sign    desktop → phone  {message} → {signature} (base64 ASN.1)
///   unlock.forget  desktop → phone  {} → {}
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../services/link/link.dart';

const _channel = MethodChannel('palt/unlock');

void setupUnlock(ProviderContainer c) {
  final links = c.read(linkManagerProvider);

  void answer(String type, Future<Map<String, dynamic>> Function(Link l, Map<String, dynamic> b) work) {
    links.handle(type, (l, p) {
      unawaited(() async {
        Map<String, dynamic> reply;
        try {
          reply = await work(l, p.body ?? const {});
        } on PlatformException catch (e) {
          reply = {'error': e.message ?? e.code};
        } catch (e) {
          reply = {'error': e.toString()};
        }
        await l.send('$type.reply', id: p.id, body: reply).catchError((Object _) {});
      }());
    });
  }

  answer(
      'unlock.setup',
      (l, _) async => {
            'publicKey': await _channel.invokeMethod<String>('setup', {'computer': l.peer.id}),
          });
  answer(
      'unlock.sign',
      (l, b) async => {
            'signature': await _channel.invokeMethod<String>('sign', {
              'computer': l.peer.id,
              'name': l.peer.name,
              'message': b['message'] as String? ?? '',
            }),
          });
  answer('unlock.forget', (l, _) async {
    await _channel.invokeMethod('forget', {'computer': l.peer.id});
    return {};
  });
}
