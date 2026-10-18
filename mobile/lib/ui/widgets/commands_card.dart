/// commands_card.dart — "Run on <computer>": the commands a paired computer
/// offers (Lock, Suspend, Mute and the user's own), one tap each.
///
/// Packets: commands.list / commands.run (desktop/commands_app.go).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/peer.dart';
import '../../providers/link_provider.dart';
import '../../theme/app_theme.dart';

class CommandsCard extends ConsumerStatefulWidget {
  final Peer peer;
  const CommandsCard({super.key, required this.peer});

  @override
  ConsumerState<CommandsCard> createState() => _CommandsCardState();
}

class _CommandsCardState extends ConsumerState<CommandsCard> {
  List<({String id, String name})>? _commands;
  String? _running;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final l = ref.read(linkManagerProvider).byName(widget.peer.deviceName);
    if (l == null || !l.supports('commands.list')) return;
    try {
      final p = await l.request('commands.list');
      final list = [
        for (final c in (p.body?['commands'] as List?) ?? const [])
          (id: (c as Map)['id'] as String, name: c['name'] as String),
      ];
      if (mounted) setState(() => _commands = list);
    } catch (_) {}
  }

  Future<void> _run(String id, String name) async {
    final l = ref.read(linkManagerProvider).byName(widget.peer.deviceName);
    if (l == null) return;
    setState(() => _running = id);
    String msg;
    try {
      final p = await l.request('commands.run', body: {'id': id}, timeout: const Duration(seconds: 70));
      final err = p.body?['error'] as String?;
      final out = (p.body?['output'] as String?)?.trim() ?? '';
      msg = err != null && err.isNotEmpty ? '$name failed: $err' : (out.isEmpty ? '$name: done' : out);
    } catch (e) {
      msg = '$name failed: $e';
    }
    if (!mounted) return;
    setState(() => _running = null);
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(msg, maxLines: 6, overflow: TextOverflow.ellipsis)));
  }

  static IconData _icon(String id) => switch (id) {
        'lock' => Icons.lock_rounded,
        'suspend' => Icons.bedtime_rounded,
        'mute' => Icons.volume_off_rounded,
        _ => Icons.terminal_rounded,
      };

  @override
  Widget build(BuildContext context) {
    final commands = _commands;
    if (commands == null || commands.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(left: 4, bottom: 10),
            child: Text('Run on ${widget.peer.deviceName}',
                style: const TextStyle(color: kTextSecondary, fontSize: 12.5, fontWeight: FontWeight.w600)),
          ),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final c in commands)
                Material(
                  type: MaterialType.transparency,
                  child: Ink(
                    decoration: BoxDecoration(
                      gradient: kTileGradient,
                      borderRadius: BorderRadius.circular(999),
                      border: Border.all(color: kHairline),
                    ),
                    child: InkWell(
                      borderRadius: BorderRadius.circular(999),
                      onTap: _running == null ? () => _run(c.id, c.name) : null,
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                        child: Row(mainAxisSize: MainAxisSize.min, children: [
                          _running == c.id
                              ? const SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(strokeWidth: 2, color: kAccent))
                              : Icon(_icon(c.id), size: 16, color: kAccent),
                          const SizedBox(width: 8),
                          Text(c.name, style: const TextStyle(color: kText, fontSize: 13, fontWeight: FontWeight.w500)),
                        ]),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
