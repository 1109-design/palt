/// calls.dart — tells paired computers about calls so they can show who's
/// calling, pause media and note missed calls; mutes the ringer on request.
///
/// Computers can also read the call log, see the SIMs, and place, answer
/// and end calls (the call's audio stays on the phone).
///
/// Packet types (must match desktop/calls_app.go). Requests carry an id and
/// are answered by "<type>.reply" with the same id.
///   call         phone → desktop  {event: ringing|talking|missed|ended, number,
///                                  name, photo?, direction, since}
///   call.mute    desktop → phone  {}
///   call.place   desktop → phone  {number, sim} → {error?}
///   call.answer  desktop → phone  {} → {error?}
///   call.end     desktop → phone  {} → {error?}
///   calls.log    desktop → phone  {before, limit} → {calls}
///   phone.sims   desktop → phone  {} → {sims: [{id, slot, name, carrier}]}
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../services/link/link.dart';

const _channel = MethodChannel('palt/calls');

void setupCalls(ProviderContainer c) {
  final links = c.read(linkManagerProvider);
  _channel.setMethodCallHandler((call) async {
    if (call.method == 'call') {
      links.broadcast('call', body: Map<String, dynamic>.from(call.arguments as Map));
    }
  });
  links.handle('call.mute', (l, p) => _channel.invokeMethod('mute'));

  void answer(String type, Future<Map<String, dynamic>> Function(Map<String, dynamic> b) work) {
    links.handle(type, (l, p) {
      unawaited(() async {
        Map<String, dynamic> reply;
        try {
          reply = await work(p.body ?? const {});
        } on PlatformException catch (e) {
          reply = {'error': e.message ?? e.code};
        } catch (e) {
          reply = {'error': e.toString()};
        }
        await l.send('$type.reply', id: p.id, body: reply).catchError((Object _) {});
      }());
    });
  }

  List<Map<String, dynamic>> maps(Object? list) =>
      ((list as List?) ?? const []).map((m) => Map<String, dynamic>.from(m as Map)).toList();

  answer('call.place', (b) async {
    await _channel.invokeMethod('place', {'number': b['number'] ?? '', 'sim': (b['sim'] as num?)?.toInt() ?? 0});
    return {};
  });
  answer('call.answer', (_) async {
    await _channel.invokeMethod('answer');
    return {};
  });
  answer('call.end', (_) async {
    await _channel.invokeMethod('end');
    return {};
  });
  answer('calls.log', (b) async => {
        'calls': maps(await _channel.invokeMethod('log', {
          'before': (b['before'] as num?)?.toInt() ?? 0,
          'limit': (b['limit'] as num?)?.toInt() ?? 80,
        })),
      });
  answer('phone.sims', (_) async => {'sims': maps(await _channel.invokeMethod('sims'))});

  // A computer linking up mid-call learns about the call in progress.
  links.changes.listen((_) async {
    _told.removeWhere((l) => l.isClosed);
    final now = await _channel.invokeMethod('current');
    if (now == null) {
      _told.clear();
      return;
    }
    for (final l in links.links) {
      if (l.supports('call') && !_told.contains(l)) {
        _told.add(l);
        l.send('call', body: Map<String, dynamic>.from(now as Map)).catchError((Object _) {});
      }
    }
  });
}

/// Links already told about the call in progress.
final _told = <Link>{};
