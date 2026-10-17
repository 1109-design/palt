/// device_screen.dart — one peer, full screen (Beacon).
///
/// Opened by tapping a PeerCard. Shows the shared history with that device
/// as a thread (outgoing right in amber, incoming left in green) and a
/// composer for text messages and files. If the peer drops off the network
/// the thread stays readable but sending is disabled until it returns.
/// Auto-copy (incoming texts → clipboard) is opt-in per device and only
/// offered for trusted devices; "Send clipboard" is always a manual tap.
library;

import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:open_filex/open_filex.dart';

import '../../models/history_entry.dart';
import '../../models/peer.dart';
import '../../providers/alias_provider.dart';
import '../../providers/auto_copy_provider.dart';
import '../../providers/history_provider.dart';
import '../../providers/peer_provider.dart';
import '../../services/transfer_service.dart';
import '../../theme/app_theme.dart';
import '../../utils/media.dart';
import '../../utils/os_icons.dart';
import '../widgets/peer_card.dart';
import '../widgets/route_badge.dart';

/// Broadcast name of the device whose screen is open, so the home screen can
/// skip toasting messages that are already visible in the thread.
final openDeviceNameProvider = StateProvider<String?>((ref) => null);

class DeviceScreen extends ConsumerStatefulWidget {
  /// The device as last seen; kept so the screen survives it going offline.
  final Peer peer;

  const DeviceScreen({super.key, required this.peer});

  @override
  ConsumerState<DeviceScreen> createState() => _DeviceScreenState();
}

class _DeviceScreenState extends ConsumerState<DeviceScreen> {
  final _controller = TextEditingController();
  final _focus = FocusNode();
  bool _sending = false;
  String? _sendError;
  late final StateController<String?> _openDevice;

  @override
  void initState() {
    super.initState();
    _openDevice = ref.read(openDeviceNameProvider.notifier);
    WidgetsBinding.instance.addPostFrameCallback(
        (_) => _openDevice.state = widget.peer.deviceName);
    _controller.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    // Riverpod forbids provider writes during unmount; defer by a microtask.
    final openDevice = _openDevice;
    final name = widget.peer.deviceName;
    Future.microtask(() {
      if (openDevice.state == name) openDevice.state = null;
    });
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  Peer? _livePeer(List<Peer> peers) {
    for (final p in peers) {
      if (p.deviceName == widget.peer.deviceName) return p;
    }
    return null;
  }

  Future<void> _submit(Peer peer) async {
    final text = _controller.text;
    if (text.trim().isEmpty || _sending) return;
    await _send(peer, text, clearDraft: true);
  }

  Future<void> _sendClipboard(Peer peer) async {
    if (_sending) return;
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text ?? '';
    if (text.trim().isEmpty) {
      setState(() => _sendError = 'Clipboard is empty.');
      return;
    }
    if (utf8.encode(text).length > kMaxTextBytes) {
      setState(() => _sendError =
          'Clipboard is over ${kMaxTextBytes ~/ 1024} KB. Send it as a file instead.');
      return;
    }
    await _send(peer, text, clearDraft: false);
  }

  Future<void> _toggleAutoCopy(String name, bool paired, bool enabled) async {
    if (!enabled && !paired) {
      ScaffoldMessenger.of(context).hideCurrentSnackBar();
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Auto-copy works for paired devices only. Pair from the link icon on the home screen.'),
      ));
      return;
    }
    ref.read(autoCopyProvider.notifier).set(name, !enabled);
    ScaffoldMessenger.of(context).hideCurrentSnackBar();
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(!enabled
          ? 'Messages from this device will be copied to the clipboard'
          : 'Auto-copy off'),
    ));
  }

  Future<void> _send(Peer peer, String text, {required bool clearDraft}) async {
    setState(() {
      _sending = true;
      _sendError = null;
    });
    try {
      await ref.read(transferServiceProvider).sendText(peer, text);
      if (clearDraft) _controller.clear();
    } catch (e) {
      setState(() => _sendError = e.toString());
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final peers = ref.watch(peerListProvider).valueOrNull ?? const [];
    final live = _livePeer(peers);
    final online = live != null;
    final peer = live ?? widget.peer;

    final alias = ref.watch(aliasProvider)[peer.deviceName];
    final displayName = (alias != null && alias.isNotEmpty) ? alias : peer.deviceName;
    final osInfo = getOsInfo(peer.os);
    final paired = peer.paired;
    final autoCopy = ref.watch(autoCopyProvider).contains(peer.deviceName);

    // History is newest-first, which is exactly what a reversed list wants.
    final thread = _groupTransfers(ref
        .watch(historyProvider)
        .where((e) => e.partnerName == peer.deviceName)
        .toList());

    final progress = ref.watch(transferProgressProvider);

    final draftBytes = utf8.encode(_controller.text).length;
    final tooLong = draftBytes > kMaxTextBytes;
    final canSend =
        online && !_sending && _controller.text.trim().isNotEmpty && !tooLong;

    final notice = !online
        ? '$displayName is offline. Sending resumes when it reappears.'
        : tooLong
            ? 'Too long for a message (${(draftBytes / 1024).toStringAsFixed(1)} of ${kMaxTextBytes ~/ 1024} KB). Send it as a file instead.'
            : _sendError;

    return Scaffold(
      appBar: AppBar(
        titleSpacing: 0,
        actions: [
          IconButton(
            tooltip: autoCopy ? 'Auto-copy on' : 'Auto-copy off',
            icon: Icon(
              autoCopy ? Icons.content_paste_rounded : Icons.content_paste_off_rounded,
              color: autoCopy ? kAccent : kTextSecondary,
            ),
            onPressed: () =>
                _toggleAutoCopy(peer.deviceName, paired, autoCopy),
          ),
          const SizedBox(width: 4),
        ],
        title: Row(
          children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                color: kSurfaceRaised,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: kHairline),
              ),
              child: Icon(osInfo.icon, color: kTextSecondary, size: 20),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(displayName,
                      style: display(fontSize: 17),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis),
                  const SizedBox(height: 2),
                  Row(
                    children: [
                      Container(
                        width: 7,
                        height: 7,
                        decoration: BoxDecoration(
                          color: online ? kOk : kTextFaint,
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: 6),
                      if (online) ...[
                        RouteBadge(route: peer.via),
                        const SizedBox(width: 6),
                      ],
                      Flexible(
                        child: Text(
                          !online
                              ? 'offline'
                              : peer.viaRelay
                                  ? 'through relay'
                                  : peer.ipAddress,
                          style: mono(fontSize: 11),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            if (progress != null) _ProgressStrip(progress: progress),
            Expanded(
              child: thread.isEmpty
                  ? _EmptyThread(online: online)
                  : ListView.builder(
                      reverse: true,
                      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                      itemCount: thread.length,
                      itemBuilder: (context, i) {
                        final entry = thread[i];
                        // Reversed list: the day header sits above the
                        // oldest entry of each day, i.e. when the next
                        // (older) item is from a different day.
                        final older = i + 1 < thread.length ? thread[i + 1] : null;
                        final showDay = older == null ||
                            _dayLabel(older.timestamp) != _dayLabel(entry.timestamp);
                        return Column(
                          // Stretch so each bubble can align left/right.
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            if (showDay) _DayDivider(label: _dayLabel(entry.timestamp)),
                            _Bubble(entry: entry),
                          ],
                        );
                      },
                    ),
            ),
            if (notice != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 4, 20, 2),
                child: Text(
                  notice,
                  style: TextStyle(
                    fontSize: 12,
                    color: online ? kDanger : kTextSecondary,
                  ),
                ),
              ),
            _Composer(
              controller: _controller,
              focus: _focus,
              enabled: online,
              canSend: canSend,
              sending: _sending,
              hint: online ? 'Message $displayName' : 'Device offline',
              onAttach: () => pickAndSendFiles(ref, peer),
              onPasteSend: () => _sendClipboard(peer),
              onSend: () => _submit(peer),
            ),
          ],
        ),
      ),
    );
  }
}

/// History stores one entry per file (id "<transferId>_<name>"); merge
/// adjacent entries from the same transfer back into one bubble.
List<HistoryEntry> _groupTransfers(List<HistoryEntry> entries) {
  String? key(HistoryEntry e) {
    if (e.isText) return null;
    final i = e.id.indexOf('_');
    return i > 0 ? '${e.direction}:${e.id.substring(0, i)}' : null;
  }

  final out = <HistoryEntry>[];
  var i = 0;
  while (i < entries.length) {
    final k = key(entries[i]);
    var j = i + 1;
    while (k != null && j < entries.length && key(entries[j]) == k) {
      j++;
    }
    if (j - i == 1) {
      out.add(entries[i]);
    } else {
      // Newest-first storage reverses file order within a batch; undo that.
      final group = entries.sublist(i, j).reversed.toList();
      final failed = group.where((e) => e.status != 'completed').toList();
      out.add(HistoryEntry(
        id: k!,
        partnerName: group.first.partnerName,
        files: [for (final e in group) ...e.files],
        totalSize: group.fold(0, (sum, e) => sum + e.totalSize),
        direction: group.first.direction,
        timestamp: entries[i].timestamp,
        status: failed.isEmpty ? 'completed' : 'error',
        errorMessage: failed.isEmpty ? null : failed.first.errorMessage,
        durationMillis: group.first.durationMillis,
        route: group.first.route,
      ));
    }
    i = j;
  }
  return out;
}

/// Where a thread file can be opened from, if anywhere. Received files fall
/// back to Downloads/PALT by name (older entries didn't record a path); sent
/// files fall back to the preview kept for pictures.
List<String> _fileCandidates(HistoryFile f, bool outgoing) => [
      if (f.path != null) f.path!,
      if (!outgoing && f.path == null) '$kPaltDownloadDir/${safeFileName(f.name)}',
      if (outgoing && f.preview != null) f.preview!,
    ];

bool _canOpen(HistoryFile f, bool outgoing) => _fileCandidates(f, outgoing).isNotEmpty;

Future<void> _openFile(BuildContext context, HistoryFile f, bool outgoing) async {
  final messenger = ScaffoldMessenger.of(context);
  OpenResult? last;
  for (final path in _fileCandidates(f, outgoing)) {
    last = await OpenFilex.open(path);
    if (last.type == ResultType.done) return;
    if (last.type != ResultType.fileNotFound) break;
  }
  messenger.showSnackBar(SnackBar(
    content: Text(last == null || last.type == ResultType.fileNotFound
        ? '${f.name} is no longer on this phone'
        : 'Could not open ${f.name}: ${last.message}'),
  ));
}

/// "Today" / "Yesterday" / "Mon, 14 Jul"
String _dayLabel(DateTime t) {
  final d = t.toLocal();
  final now = DateTime.now();
  final diff = DateTime(now.year, now.month, now.day)
      .difference(DateTime(d.year, d.month, d.day))
      .inDays;
  if (diff == 0) return 'Today';
  if (diff == 1) return 'Yesterday';
  const weekdays = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
  const months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
  ];
  return '${weekdays[d.weekday - 1]}, ${d.day} ${months[d.month - 1]}';
}

String _formatTime(DateTime t) {
  final l = t.toLocal();
  return '${l.hour.toString().padLeft(2, '0')}:${l.minute.toString().padLeft(2, '0')}';
}

String _formatSize(int bytes) {
  if (bytes <= 0) return '0 B';
  const suffixes = ['B', 'KB', 'MB', 'GB', 'TB'];
  var i = 0;
  double size = bytes.toDouble();
  while (size >= 1024 && i < suffixes.length - 1) {
    size /= 1024;
    i++;
  }
  return '${size.toStringAsFixed(1)} ${suffixes[i]}';
}

class _DayDivider extends StatelessWidget {
  final String label;
  const _DayDivider({required this.label});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Row(
        children: [
          const Expanded(child: Divider(color: kHairline, height: 1)),
          const SizedBox(width: 12),
          Text(label.toUpperCase(), style: overline()),
          const SizedBox(width: 12),
          const Expanded(child: Divider(color: kHairline, height: 1)),
        ],
      ),
    );
  }
}

/// A picture or video frame shown inline in the thread, WhatsApp-style.
/// Falls back to the plain file row when it can't be loaded.
class _ImageTile extends StatelessWidget {
  final HistoryFile file;
  final bool outgoing;
  final Widget fallback;
  final VoidCallback onTap;
  const _ImageTile(
      {required this.file, required this.outgoing, required this.fallback, required this.onTap});

  Widget _frame(BuildContext context, String src, {Widget? overlay}) {
    final dpr = MediaQuery.of(context).devicePixelRatio;
    return GestureDetector(
      onTap: onTap,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(10),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: 320, minWidth: 120),
          child: Stack(
            alignment: Alignment.center,
            children: [
              Image.file(
                File(src),
                cacheWidth: (260 * dpr).round(),
                fit: BoxFit.cover,
                gaplessPlayback: true,
                errorBuilder: (_, __, ___) => fallback,
              ),
              if (overlay != null) Positioned.fill(child: overlay),
            ],
          ),
        ),
      ),
    );
  }

  Widget _videoOverlay(Duration? duration) => Stack(
        children: [
          Center(
            child: Container(
              width: 46,
              height: 46,
              decoration: BoxDecoration(
                color: kBg.withValues(alpha: 0.65),
                shape: BoxShape.circle,
                border: Border.all(color: kText.withValues(alpha: 0.25)),
              ),
              child: const Icon(Icons.play_arrow_rounded, color: kText, size: 28),
            ),
          ),
          if (duration != null && duration > Duration.zero)
            Positioned(
              left: 8,
              bottom: 8,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                decoration: BoxDecoration(
                  color: kBg.withValues(alpha: 0.7),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(formatDuration(duration), style: mono(fontSize: 10.5, color: kText)),
              ),
            ),
        ],
      );

  @override
  Widget build(BuildContext context) {
    if (isVideoName(file.name)) {
      final video = outgoing
          ? file.path
          : (file.path ?? '$kPaltDownloadDir/${safeFileName(file.name)}');
      return FutureBuilder<VideoThumb?>(
        future: video == null ? Future.value(null) : videoThumb(video),
        builder: (context, snap) {
          final thumb = snap.data;
          if (thumb != null) {
            return _frame(context, thumb.path, overlay: _videoOverlay(thumb.duration));
          }
          // Sent video whose temporary copy is gone: the frame kept at send time.
          if (snap.connectionState == ConnectionState.done && outgoing && file.preview != null) {
            return _frame(context, file.preview!, overlay: _videoOverlay(null));
          }
          if (snap.connectionState == ConnectionState.done) return fallback;
          return Container(
            width: 220,
            height: 140,
            decoration: BoxDecoration(
              color: kSurfaceRaised,
              borderRadius: BorderRadius.circular(10),
            ),
          );
        },
      );
    }

    // Sent pictures: the kept preview outlives the picker's temporary copy.
    final src = outgoing
        ? (file.preview ?? file.path)
        : (file.path ?? '$kPaltDownloadDir/${safeFileName(file.name)}');
    if (src == null) return fallback;
    return _frame(context, src);
  }
}

class _Bubble extends ConsumerWidget {
  final HistoryEntry entry;
  const _Bubble({required this.entry});

  bool _isPicture(HistoryFile f) => isMediaName(f.name) && entry.status == 'completed';

  Widget _fileRow(BuildContext context, HistoryFile f, Color tint, bool outgoing, bool failed) {
    final canOpen = !failed && _canOpen(f, outgoing);
    return InkWell(
      borderRadius: BorderRadius.circular(8),
      onTap: canOpen ? () => _openFile(context, f, outgoing) : null,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 6, horizontal: 2),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.insert_drive_file_outlined, size: 17, color: tint),
            const SizedBox(width: 8),
            Flexible(
              child: Text(
                f.name,
                style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600, color: kText),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 10),
            Text(_formatSize(f.size), style: mono(fontSize: 11)),
            if (canOpen) ...[
              const SizedBox(width: 6),
              const Icon(Icons.chevron_right_rounded, size: 16, color: kTextFaint),
            ],
          ],
        ),
      ),
    );
  }

  void _showActions(BuildContext context, WidgetRef ref) {
    final outgoing = entry.direction == 'outgoing';
    showModalBottomSheet(
      context: context,
      builder: (sheet) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (entry.isText)
              ListTile(
                leading: const Icon(Icons.copy_rounded),
                title: const Text('Copy'),
                onTap: () {
                  Navigator.pop(sheet);
                  Clipboard.setData(ClipboardData(text: entry.text ?? ''));
                  ScaffoldMessenger.of(context)
                      .showSnackBar(const SnackBar(content: Text('Copied')));
                },
              ),
            if (!entry.isText && entry.files.length == 1 && _canOpen(entry.files.first, outgoing))
              ListTile(
                leading: const Icon(Icons.open_in_new_rounded),
                title: const Text('Open'),
                onTap: () {
                  Navigator.pop(sheet);
                  _openFile(context, entry.files.first, outgoing);
                },
              ),
            ListTile(
              leading: const Icon(Icons.delete_outline_rounded, color: kDanger),
              title: const Text('Delete', style: TextStyle(color: kDanger)),
              subtitle: entry.isText
                  ? null
                  : const Text('Removes it from this chat. Files stay on the phone.',
                      style: TextStyle(fontSize: 12, color: kTextFaint)),
              onTap: () {
                Navigator.pop(sheet);
                ref.read(historyProvider.notifier).removeEntry(entry);
              },
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final outgoing = entry.direction == 'outgoing';
    final failed = entry.status != 'completed';
    final tint = failed ? kDanger : (outgoing ? kAccent : kOk);
    final maxWidth = MediaQuery.of(context).size.width * 0.78;
    final picturesOnly =
        !entry.isText && entry.files.isNotEmpty && entry.files.every(_isPicture);

    final body = entry.isText
        ? SelectableText(
            entry.text ?? '',
            style: const TextStyle(fontSize: 14.5, color: kText, height: 1.35),
          )
        : Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              for (final f in entry.files)
                if (_isPicture(f))
                  Padding(
                    padding: EdgeInsets.only(bottom: f == entry.files.last ? 0 : 4),
                    child: _ImageTile(
                      file: f,
                      outgoing: outgoing,
                      onTap: () => _openFile(context, f, outgoing),
                      fallback: _fileRow(context, f, tint, outgoing, failed),
                    ),
                  )
                else
                  _fileRow(context, f, tint, outgoing, failed),
            ],
          );

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Column(
        crossAxisAlignment:
            outgoing ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          GestureDetector(
            onLongPress: () => _showActions(context, ref),
            child: Container(
              constraints: BoxConstraints(maxWidth: maxWidth),
              padding: picturesOnly
                  ? const EdgeInsets.all(4)
                  : const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              decoration: BoxDecoration(
                color: tint.withValues(alpha: 0.08),
                border: Border.all(color: tint.withValues(alpha: 0.28)),
                borderRadius: BorderRadius.only(
                  topLeft: Radius.circular(outgoing ? 14 : 4),
                  topRight: Radius.circular(outgoing ? 4 : 14),
                  bottomLeft: const Radius.circular(14),
                  bottomRight: const Radius.circular(14),
                ),
              ),
              child: Material(type: MaterialType.transparency, child: body),
            ),
          ),
          const SizedBox(height: 4),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (entry.copied) ...[
                  const Icon(Icons.content_paste_rounded, size: 12, color: kTextSecondary),
                  const SizedBox(width: 4),
                  const Text('Copied to clipboard',
                      style: TextStyle(fontSize: 11, color: kTextSecondary)),
                  const SizedBox(width: 8),
                ],
                if (failed) ...[
                  const Icon(Icons.error_outline_rounded, size: 13, color: kDanger),
                  const SizedBox(width: 4),
                  GestureDetector(
                    onTap: entry.errorMessage == null
                        ? null
                        : () => ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(entry.errorMessage!)),
                            ),
                    child: const Text('Not delivered',
                        style: TextStyle(fontSize: 11, color: kDanger)),
                  ),
                  const SizedBox(width: 8),
                ],
                Text(_formatTime(entry.timestamp),
                    style: mono(fontSize: 10.5, color: kTextFaint)),
                if (entry.route != null)
                  Text(' · ${routeLabel(entry.route)}',
                      style: mono(
                          fontSize: 10.5,
                          color: entry.route == 'relay' ? kRelay : kTextFaint)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyThread extends StatelessWidget {
  final bool online;
  const _EmptyThread({required this.online});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.forum_outlined, size: 48, color: kTextFaint),
            const SizedBox(height: 14),
            Text('Nothing shared yet', style: display(fontSize: 18)),
            const SizedBox(height: 6),
            const Text(
              'Type a message below, or tap the clip to send files.',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 13, color: kTextSecondary, height: 1.5),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProgressStrip extends StatelessWidget {
  final TransferProgress progress;
  const _ProgressStrip({required this.progress});

  @override
  Widget build(BuildContext context) {
    final isError = progress.status == TransferStatus.error;
    final isDone = progress.status == TransferStatus.completed;
    final isWaiting = progress.status == TransferStatus.waiting;
    final color = isError ? kDanger : (isDone ? kOk : kAccent);
    final value = progress.total > 0 ? progress.written / progress.total : 0.0;

    final label = isError
        ? (progress.error ?? 'Transfer failed')
        : isDone
            ? 'Transfer complete'
            : isWaiting
                ? 'Waiting for the other device…'
                : '${progress.currentFileName ?? 'Transferring'} · file ${progress.sentItems ?? 1} of ${progress.totalItems ?? 1}';

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
      decoration: const BoxDecoration(
        color: kSurface,
        border: Border(bottom: BorderSide(color: kHairline)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(label,
                    style: TextStyle(fontSize: 12.5, color: isError ? kDanger : kText),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis),
              ),
              if (progress.route != null && !isError) ...[
                const SizedBox(width: 8),
                RouteBadge(route: progress.route),
              ],
            ],
          ),
          if (!isError) ...[
            const SizedBox(height: 8),
            ClipRRect(
              borderRadius: BorderRadius.circular(999),
              child: LinearProgressIndicator(
                value: isWaiting ? null : (isDone ? 1.0 : value),
                minHeight: 4,
                backgroundColor: kHairline,
                valueColor: AlwaysStoppedAnimation<Color>(color),
              ),
            ),
            if (!isWaiting) ...[
              const SizedBox(height: 6),
              Row(
                children: [
                  Text(
                    '${_formatSize(isDone ? progress.total : progress.written)} / ${_formatSize(progress.total)}',
                    style: mono(fontSize: 10.5, color: kTextFaint),
                  ),
                  const Spacer(),
                  Text(
                    isDone
                        ? 'done'
                        : '${progress.speed != null ? '${_formatSize(progress.speed!.round())}/s' : '—'} · ${(value * 100).round()}%',
                    style: mono(fontSize: 10.5),
                  ),
                ],
              ),
            ],
          ],
        ],
      ),
    );
  }
}

class _Composer extends StatelessWidget {
  final TextEditingController controller;
  final FocusNode focus;
  final bool enabled;
  final bool canSend;
  final bool sending;
  final String hint;
  final VoidCallback onAttach;
  final VoidCallback onPasteSend;
  final VoidCallback onSend;

  const _Composer({
    required this.controller,
    required this.focus,
    required this.enabled,
    required this.canSend,
    required this.sending,
    required this.hint,
    required this.onAttach,
    required this.onPasteSend,
    required this.onSend,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: kHairline)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          IconButton(
            icon: const Icon(Icons.attach_file_rounded),
            color: kTextSecondary,
            tooltip: 'Send files',
            onPressed: enabled ? onAttach : null,
          ),
          IconButton(
            icon: const Icon(Icons.content_paste_go_rounded),
            color: kTextSecondary,
            tooltip: 'Send clipboard',
            onPressed: enabled && !sending ? onPasteSend : null,
          ),
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: kSurface,
                borderRadius: BorderRadius.circular(14),
                border: Border.all(color: kHairline),
              ),
              padding: const EdgeInsets.symmetric(horizontal: 14),
              child: TextField(
                controller: controller,
                focusNode: focus,
                enabled: enabled,
                minLines: 1,
                maxLines: 6,
                keyboardType: TextInputType.multiline,
                textCapitalization: TextCapitalization.sentences,
                style: const TextStyle(fontSize: 15, color: kText),
                decoration: InputDecoration(
                  hintText: hint,
                  hintStyle: const TextStyle(color: kTextFaint),
                  border: InputBorder.none,
                  enabledBorder: InputBorder.none,
                  focusedBorder: InputBorder.none,
                  disabledBorder: InputBorder.none,
                  filled: false,
                  isDense: true,
                  contentPadding: const EdgeInsets.symmetric(vertical: 12),
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          SizedBox(
            width: 46,
            height: 46,
            child: Material(
              color: canSend ? kAccent : kSurfaceRaised,
              borderRadius: BorderRadius.circular(12),
              child: InkWell(
                borderRadius: BorderRadius.circular(12),
                onTap: canSend ? onSend : null,
                child: Center(
                  child: sending
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                              strokeWidth: 2, color: kTextFaint),
                        )
                      : Icon(Icons.arrow_upward_rounded,
                          size: 21, color: canSend ? kOnAccent : kTextFaint),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
