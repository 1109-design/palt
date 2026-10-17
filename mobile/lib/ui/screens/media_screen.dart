/// media_screen.dart — remote control for what a computer is playing:
/// each player with its art, a seek bar and transport buttons, and the
/// computer's volume.
library;

import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/media.dart';
import '../../providers/link_provider.dart';
import '../../theme/app_theme.dart';

class MediaScreen extends ConsumerStatefulWidget {
  final String device;
  const MediaScreen({super.key, required this.device});

  @override
  ConsumerState<MediaScreen> createState() => _MediaScreenState();
}

class _MediaScreenState extends ConsumerState<MediaScreen> {
  Timer? _tick;
  double? _volumeDrag;

  @override
  void initState() {
    super.initState();
    // Seek bars move on their own while playing.
    _tick = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _tick?.cancel();
    super.dispose();
  }

  void _act(String player, String action, [double value = 0]) =>
      sendMediaAction(ref.read(linkManagerProvider), widget.device, player, action, value);

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(peerMediaProvider)[widget.device];
    final players = [...?state?.players]..sort((a, b) => (b.playing ? 1 : 0) - (a.playing ? 1 : 0));
    return Scaffold(
      appBar: AppBar(title: Text('Media on ${widget.device}', style: display(fontSize: 18))),
      body: state == null
          ? Center(child: Text('Waiting for ${widget.device}…', style: mono(color: kTextFaint)))
          : ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                if (players.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 48),
                    child: Column(children: [
                      const Icon(Icons.music_off_rounded, size: 40, color: kTextFaint),
                      const SizedBox(height: 10),
                      Text('Nothing is playing on ${widget.device}',
                          style: const TextStyle(color: kTextSecondary)),
                    ]),
                  ),
                for (final p in players) ...[
                  _PlayerCard(player: p, state: state, act: _act),
                  const SizedBox(height: 12),
                ],
                if (state.volume >= 0) _volumeCard(state),
              ],
            ),
    );
  }

  Widget _volumeCard(MediaState state) {
    final v = _volumeDrag ?? state.volume.toDouble();
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 8, 8),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: kHairline),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('${widget.device.toUpperCase()} VOLUME', style: overline()),
          Row(children: [
            IconButton(
              tooltip: state.muted ? 'Unmute' : 'Mute',
              onPressed: () => _act('', 'mute'),
              icon: Icon(state.muted ? Icons.volume_off_rounded : Icons.volume_up_rounded,
                  color: state.muted ? kDanger : kText),
            ),
            Expanded(
              child: Slider(
                value: v.clamp(0, 100),
                max: 100,
                onChanged: (x) => setState(() => _volumeDrag = x),
                onChangeEnd: (x) {
                  _act('', 'volume', x.roundToDouble());
                  Future.delayed(const Duration(seconds: 1), () {
                    if (mounted) setState(() => _volumeDrag = null);
                  });
                },
              ),
            ),
            SizedBox(width: 40, child: Text('${v.round()}%', style: mono(fontSize: 12))),
          ]),
        ],
      ),
    );
  }
}

class _PlayerCard extends StatefulWidget {
  final MediaPlayer player;
  final MediaState state;
  final void Function(String player, String action, [double value]) act;
  const _PlayerCard({required this.player, required this.state, required this.act});

  @override
  State<_PlayerCard> createState() => _PlayerCardState();
}

class _PlayerCardState extends State<_PlayerCard> {
  double? _seekDrag;

  @override
  Widget build(BuildContext context) {
    final p = widget.player;
    final pos = _seekDrag ?? widget.state.positionOf(p).toDouble();
    final canSeek = p.canSeek && p.length > 0;
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: p.playing ? kAccent.withValues(alpha: 0.5) : kHairline),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            _Art(url: p.art),
            const SizedBox(width: 14),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text(p.name.toUpperCase(), style: overline(color: p.playing ? kAccent : kTextFaint)),
                const SizedBox(height: 4),
                Text(p.title.isEmpty ? 'Unknown' : p.title,
                    style: display(fontSize: 16), maxLines: 2, overflow: TextOverflow.ellipsis),
                if (p.artist.isNotEmpty)
                  Text(p.artist,
                      style: const TextStyle(color: kTextSecondary, fontSize: 13),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis),
              ]),
            ),
          ]),
          if (canSeek) ...[
            const SizedBox(height: 6),
            Slider(
              value: pos.clamp(0, p.length.toDouble()),
              max: p.length.toDouble(),
              onChanged: (x) => setState(() => _seekDrag = x),
              onChangeEnd: (x) {
                widget.act(p.id, 'seek', x.roundToDouble());
                Future.delayed(const Duration(seconds: 1), () {
                  if (mounted) setState(() => _seekDrag = null);
                });
              },
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              child: Row(children: [
                Text(_time(pos.toInt()), style: mono(fontSize: 11, color: kTextFaint)),
                const Spacer(),
                Text(_time(p.length), style: mono(fontSize: 11, color: kTextFaint)),
              ]),
            ),
          ] else
            const SizedBox(height: 10),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              IconButton(
                iconSize: 32,
                onPressed: p.canPrev ? () => widget.act(p.id, 'previous') : null,
                icon: const Icon(Icons.skip_previous_rounded),
              ),
              const SizedBox(width: 12),
              IconButton.filled(
                iconSize: 38,
                onPressed: () => widget.act(p.id, 'playpause'),
                icon: Icon(p.playing ? Icons.pause_rounded : Icons.play_arrow_rounded),
              ),
              const SizedBox(width: 12),
              IconButton(
                iconSize: 32,
                onPressed: p.canNext ? () => widget.act(p.id, 'next') : null,
                icon: const Icon(Icons.skip_next_rounded),
              ),
            ],
          ),
        ],
      ),
    );
  }

  static String _time(int ms) {
    final s = ms ~/ 1000;
    final h = s ~/ 3600, m = (s % 3600) ~/ 60, sec = s % 60;
    final mm = h > 0 ? m.toString().padLeft(2, '0') : '$m';
    return '${h > 0 ? '$h:' : ''}$mm:${sec.toString().padLeft(2, '0')}';
  }
}

/// Cover art from a web URL or a data: URL, or a placeholder.
class _Art extends StatelessWidget {
  final String url;
  const _Art({required this.url});

  @override
  Widget build(BuildContext context) {
    Widget img;
    if (url.startsWith('data:')) {
      final comma = url.indexOf(',');
      img = Image.memory(base64Decode(url.substring(comma + 1)), fit: BoxFit.cover, gaplessPlayback: true);
    } else if (url.startsWith('http')) {
      img = Image.network(url, fit: BoxFit.cover, errorBuilder: (_, __, ___) => const _NoArt());
    } else {
      img = const _NoArt();
    }
    return ClipRRect(borderRadius: BorderRadius.circular(10), child: SizedBox(width: 72, height: 72, child: img));
  }
}

class _NoArt extends StatelessWidget {
  const _NoArt();

  @override
  Widget build(BuildContext context) => Container(
        color: kSurfaceRaised,
        child: const Icon(Icons.music_note_rounded, color: kTextFaint, size: 30),
      );
}
