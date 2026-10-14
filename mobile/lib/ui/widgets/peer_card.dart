/// peer_card.dart — a discovered peer (Beacon).
///
/// Dark surface card: OS tile, display-type name, mono address readout,
/// one amber Send action. Tapping the card opens the device screen (thread +
/// composer); the amber square is a shortcut straight to the file picker.
library;

import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart' as fp;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'dart:io';

import '../../models/peer.dart';
import '../../utils/os_icons.dart';
import '../../services/transfer_service.dart';
import '../../theme/app_theme.dart';

import '../../providers/alias_provider.dart';
import '../screens/device_screen.dart';

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
                          Text(peer.ipAddress, style: mono(fontSize: 11.5)),
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
