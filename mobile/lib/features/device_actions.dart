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

/// Shares this phone's Wi-Fi with a computer. Android never hands apps a
/// saved password, so the phone shows its network as a QR code and the
/// computer reads it from a screenshot sent through PALT.
Future<void> shareWifiTo(BuildContext context, Peer peer) => showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Share Wi-Fi'),
        content: Text(
          '1. In Wi-Fi settings, tap your network, then QR code.\n'
          '2. Take a screenshot.\n'
          '3. Share the screenshot to PALT → ${peer.deviceName}.\n\n'
          '${peer.deviceName} will ask to join.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              Navigator.pop(ctx);
              const MethodChannel('palt/device').invokeMethod('openWifiSettings');
            },
            child: const Text('Open Wi-Fi settings'),
          ),
        ],
      ),
    );

/// Internet between this phone and a computer, whichever way it's needed:
/// asks the computer to share when this phone is offline, else offers this
/// phone's data (hotspot or USB tethering).
Future<void> shareInternetWith(BuildContext context, WidgetRef ref, Peer peer, {required bool phoneOnline}) async {
  if (!phoneOnline) {
    final link = ref.read(linkManagerProvider).byName(peer.deviceName);
    if (link == null) return _say(context, '${peer.deviceName} isn\'t connected');
    if (!link.supports('internet.request')) return _say(context, 'Update PALT on ${peer.deviceName} for this');
    await link.send('internet.request', body: {});
    if (context.mounted) _say(context, 'Asked ${peer.deviceName} — tap Join when it offers a network');
    return;
  }
  await showDialog(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Share this phone\'s internet'),
      content: Text(
        'Turn on Mobile Hotspot (then share it with Wi-Fi → QR code), or plug in the cable '
        'and turn on USB tethering — ${peer.deviceName} uses it right away.',
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            Navigator.pop(ctx);
            const MethodChannel('palt/device').invokeMethod('openTetherSettings');
          },
          child: const Text('Open hotspot settings'),
        ),
      ],
    ),
  );
}
