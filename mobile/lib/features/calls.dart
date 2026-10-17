/// calls.dart — tells paired computers about calls so they can show who's
/// calling, pause media and note missed calls; mutes the ringer on request.
///
/// Packet types (must match desktop/calls_app.go):
///   call       phone → desktop  {event: ringing|talking|missed|ended,
///                                number, name, photo?}
///   call.mute  desktop → phone  {}
library;

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';

const _channel = MethodChannel('palt/calls');

void setupCalls(ProviderContainer c) {
  final links = c.read(linkManagerProvider);
  _channel.setMethodCallHandler((call) async {
    if (call.method == 'call') {
      links.broadcast('call', body: Map<String, dynamic>.from(call.arguments as Map));
    }
  });
  links.handle('call.mute', (l, p) => _channel.invokeMethod('mute'));
}
