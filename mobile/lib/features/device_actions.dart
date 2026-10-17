/// device_actions.dart — quick actions on a paired device, shared by the
/// home card and the device screen.
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/peer.dart';
import '../providers/link_provider.dart';
import '../services/transfer_service.dart';

void _say(BuildContext context, String msg) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(content: Text(msg)));
}

/// Sends a feature packet (ring, user_ping) over the device's link.
Future<void> sendDevicePacket(BuildContext context, WidgetRef ref, Peer peer, String type) async {
  final link = ref.read(linkManagerProvider).byName(peer.deviceName);
  if (link == null) return _say(context, '${peer.deviceName} isn\'t connected');
  if (!link.supports(type)) return _say(context, 'Update PALT on ${peer.deviceName} for this');
  await link.send(type, body: {});
  if (context.mounted) _say(context, type == 'ring' ? 'Ringing ${peer.deviceName}…' : 'Pinged ${peer.deviceName}');
}

/// Sends this phone's clipboard text as a message (it lands on the other
/// device's clipboard too when that is set up).
Future<void> sendClipboardTo(BuildContext context, WidgetRef ref, Peer peer) async {
  final data = await Clipboard.getData(Clipboard.kTextPlain);
  final text = data?.text?.trim() ?? '';
  if (!context.mounted) return;
  if (text.isEmpty) return _say(context, 'The clipboard is empty');
  try {
    await ref.read(transferServiceProvider).sendText(peer, text);
    if (context.mounted) _say(context, 'Clipboard sent');
  } catch (e) {
    if (context.mounted) _say(context, e.toString());
  }
}
