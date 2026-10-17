/// peer_card.dart — a discovered peer (Beacon).
///
/// Dark surface card: OS tile, display-type name, mono address readout,
/// one amber Send action. Tapping the card opens the device screen (thread +
/// composer); the amber square is a shortcut straight to the file picker.
library;

import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart' as fp;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/device_actions.dart';
import '../../features/phone_features.dart';
import 'device_status.dart';
import 'dart:io';

import '../../models/peer.dart';
import '../../utils/os_icons.dart';
import '../../services/transfer_service.dart';
import '../../theme/app_theme.dart';

import '../../providers/alias_provider.dart';
import '../screens/device_screen.dart';
import 'route_badge.dart';

/// Opens the system file picker and streams the chosen files to [peer].
Future<void> pickAndSendFiles(WidgetRef ref, Peer peer) async {
  try {
    final result = await fp.FilePicker.pickFiles(allowMultiple: true);
    if (result == null || result.files.isEmpty) return;
    final files = result.paths
        .where((path) => path != null)
        .map((path) => File(path!))
        .toList();
    if (files.isNotEmpty) {
      await ref.read(transferServiceProvider).sendFiles(peer, files);
    }
  } catch (e) {
    // Handled by service
  }
}

class PeerCard extends ConsumerWidget {
  final Peer peer;
  final VoidCallback? onSendFile;

  const PeerCard({super.key, required this.peer, this.onSendFile});

  void _showRenameDialog(BuildContext context, WidgetRef ref, String currentAlias) {
    final controller = TextEditingController(text: currentAlias);
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('RENAME DEVICE', style: overline()),
            const SizedBox(height: 4),
            Text(
              peer.deviceName,
              style: display(fontSize: 19),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(
            hintText: 'Nickname — leave empty to reset',
            helperText: 'Only this phone sees the nickname',
          ),
          autofocus: true,
          style: const TextStyle(color: kText),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              ref
                  .read(aliasProvider.notifier)
                  .setAlias(peer.deviceName, controller.text.trim());
              Navigator.pop(context);
            },
            child: const Text('Save'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (peer.paired) return _PairedCard(peer: peer, onSend: () => _handleSend(ref), onRename: (a) => _showRenameDialog(context, ref, a));
    final osInfo = getOsInfo(peer.os);
    final aliases = ref.watch(aliasProvider);
    final alias = aliases[peer.deviceName];
    final hasAlias = alias != null && alias.isNotEmpty;

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 20, vertical: 6),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: kHairline),
      ),
      clipBehavior: Clip.antiAlias,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => Navigator.push(
            context,
            MaterialPageRoute(builder: (_) => DeviceScreen(peer: peer)),
          ),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(
                    color: kSurfaceRaised,
                    borderRadius: BorderRadius.circular(13),
                    border: Border.all(color: kHairline),
                  ),
                  child: Icon(osInfo.icon, color: kTextSecondary, size: 25),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        hasAlias ? alias : peer.deviceName,
                        style: display(fontSize: 17),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 3),
                      Row(
                        children: [
                          RouteBadge(route: peer.via),
                          if (!peer.viaRelay) ...[
                            const SizedBox(width: 8),
                            Flexible(
                              child: Text(peer.ipAddress,
                                  style: mono(fontSize: 11.5),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis),
                            ),
                          ],
                          const SizedBox(width: 8),
                          Container(
                            width: 3,
                            height: 3,
                            decoration: const BoxDecoration(
                              color: kTextFaint,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            osInfo.label,
                            style: const TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w500,
                              color: kTextFaint,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  icon: const Icon(Icons.edit_outlined, size: 18, color: kTextFaint),
                  tooltip: 'Rename',
                  onPressed: () => _showRenameDialog(context, ref, alias ?? ''),
                  visualDensity: VisualDensity.compact,
                ),
                // Quick send: straight to the file picker
                SizedBox(
                  width: 42,
                  height: 42,
                  child: Material(
                    color: kAccent,
                    borderRadius: BorderRadius.circular(11),
                    child: InkWell(
                      borderRadius: BorderRadius.circular(11),
                      onTap: () => _handleSend(ref),
                      child: const Tooltip(
                        message: 'Send files',
                        child: Icon(Icons.north_east_rounded,
                            size: 19, color: kOnAccent),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _handleSend(WidgetRef ref) async {
    if (onSendFile != null) {
      onSendFile!();
      return;
    }
    await pickAndSendFiles(ref, peer);
  }
}

/// A paired device as a little control panel: identity, live battery and
/// network, and the actions you use most.
class _PairedCard extends ConsumerWidget {
  final Peer peer;
  final VoidCallback onSend;
  final void Function(String alias) onRename;
  const _PairedCard({required this.peer, required this.onSend, required this.onRename});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final osInfo = getOsInfo(peer.os);
    final alias = ref.watch(aliasProvider)[peer.deviceName] ?? '';
    final status = ref.watch(peerStatusProvider)[peer.deviceName];
    void open() => Navigator.push(context, MaterialPageRoute(builder: (_) => DeviceScreen(peer: peer)));

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 20, vertical: 6),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: kHairline),
      ),
      clipBehavior: Clip.antiAlias,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: open,
          onLongPress: () => onRename(alias),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 48,
                      height: 48,
                      decoration: BoxDecoration(
                        color: kSurfaceRaised,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(color: kHairline),
                      ),
                      child: Icon(osInfo.icon, color: kText, size: 24),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(alias.isNotEmpty ? alias : peer.deviceName,
                              style: display(fontSize: 18),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis),
                          const SizedBox(height: 4),
                          Row(
                            children: [
                              Container(
                                width: 7,
                                height: 7,
                                decoration: const BoxDecoration(color: kOk, shape: BoxShape.circle),
                              ),
                              const SizedBox(width: 6),
                              Text('Paired · ${osInfo.label}',
                                  style: const TextStyle(fontSize: 12, color: kTextSecondary)),
                              const SizedBox(width: 8),
                              RouteBadge(route: peer.via),
                            ],
                          ),
                        ],
                      ),
                    ),
                    const Icon(Icons.chevron_right_rounded, color: kTextFaint),
                  ],
                ),
                const SizedBox(height: 14),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                  decoration: BoxDecoration(
                    color: kBg,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: kHairline),
                  ),
                  child: StatusLine(status: status),
                ),
                const SizedBox(height: 6),
                QuickActions(actions: [
                  (icon: Icons.attach_file_rounded, label: 'Files', onTap: onSend),
                  (icon: Icons.content_paste_go_rounded, label: 'Clipboard', onTap: () => sendClipboardTo(context, ref, peer)),
                  (icon: Icons.notifications_active_rounded, label: 'Ring', onTap: () => sendDevicePacket(context, ref, peer, 'ring')),
                  (icon: Icons.waving_hand_rounded, label: 'Ping', onTap: () => sendDevicePacket(context, ref, peer, 'user_ping')),
                  (icon: Icons.chat_bubble_outline_rounded, label: 'Chat', onTap: open),
                ]),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
