/// media.dart — the media remote between this phone and paired computers.
///
/// Each side tells the other what is playing (title, art, position) and its
/// output volume; either can control the other's players and volume. This
/// phone's media comes from Android media sessions (MediaSessions.kt), which
/// the notification access PALT already has unlocks.
///
/// What a computer plays also shows in this phone's own media controls (quick
/// panel, lock screen) through a media session (RemoteMedia.kt), unless
/// turned off for that device.
///
/// Packet types (must match desktop/media_app.go):
///   media.players  either way  MediaState {players, volume, at}
///   media.action   either way  {player, action, value}
///                              player actions: playpause, play, pause, next,
///                              previous, seek (value ms), volume (value 0-1);
///                              player '': volume (value 0-100), mute (toggle)
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/device_features_provider.dart';
import '../providers/link_provider.dart';
import '../services/link/link.dart';

const _channel = MethodChannel('palt/sessions');
const _remote = MethodChannel('palt/remotemedia');

class MediaPlayer {
  final String id, name, title, artist, album, art;
  final bool playing, canNext, canPrev, canSeek, canControl;
  final int position, length; // ms; position as of MediaState.at
  final double volume; // 0-1, -1 not settable

  const MediaPlayer({
    required this.id,
    this.name = '',
    this.title = '',
    this.artist = '',
    this.album = '',
    this.art = '',
    this.playing = false,
    this.canNext = false,
    this.canPrev = false,
    this.canSeek = false,
    this.canControl = false,
    this.position = 0,
    this.length = 0,
    this.volume = -1,
  });

  factory MediaPlayer.fromJson(Map<String, dynamic> j) => MediaPlayer(
        id: j['id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        title: j['title'] as String? ?? '',
        artist: j['artist'] as String? ?? '',
        album: j['album'] as String? ?? '',
        art: j['art'] as String? ?? '',
        playing: j['playing'] as bool? ?? false,
        canNext: j['canNext'] as bool? ?? false,
        canPrev: j['canPrev'] as bool? ?? false,
        canSeek: j['canSeek'] as bool? ?? false,
        canControl: j['canControl'] as bool? ?? false,
        position: (j['position'] as num?)?.toInt() ?? 0,
        length: (j['length'] as num?)?.toInt() ?? 0,
        volume: (j['volume'] as num?)?.toDouble() ?? -1,
      );
}

class MediaState {
  final List<MediaPlayer> players;
  final int volume; // 0-100, -1 unknown
  final bool muted;
  final DateTime at; // when positions were read

  const MediaState({this.players = const [], this.volume = -1, this.muted = false, required this.at});

  factory MediaState.fromJson(Map<String, dynamic> j) {
    final v = (j['volume'] as Map?)?.cast<String, dynamic>() ?? const {};
    return MediaState(
      players: ((j['players'] as List?) ?? const [])
          .whereType<Map>()
          .map((m) => MediaPlayer.fromJson(m.cast<String, dynamic>()))
          .where((p) => p.id.isNotEmpty)
          .toList(),
      volume: (v['level'] as num?)?.toInt() ?? -1,
      muted: v['muted'] as bool? ?? false,
      // The sender's clock may differ: count from when it arrived.
      at: DateTime.now(),
    );
  }

  /// The player to show first: playing, else the first with a title.
  MediaPlayer? get main =>
      players.where((p) => p.playing).firstOrNull ?? players.where((p) => p.title.isNotEmpty).firstOrNull;

  /// [p]'s position now, counting on from [at] while it plays.
  int positionOf(MediaPlayer p) {
    if (!p.playing) return p.position;
    final now = p.position + DateTime.now().difference(at).inMilliseconds;
    return p.length > 0 ? now.clamp(0, p.length) : now;
  }
}

/// What each linked computer reported playing, by device name.
final peerMediaProvider = StateProvider<Map<String, MediaState>>((ref) => const {});

/// Controls a player (or with [player] '', the volume) on a computer.
Future<void> sendMediaAction(LinkManager links, String device, String player, String action,
    [double value = 0]) async {
  final link = links.byName(device);
  if (link == null || !link.supports('media.action')) return;
  await link.send('media.action', body: {'player': player, 'action': action, 'value': value});
}

/// Wires the media remote into the running core. Call once from startCore.
void setupMedia(ProviderContainer c) {
  final links = c.read(linkManagerProvider);

  // A computer's player in this phone's media controls: the one playing
  // (preferring the one already shown), else the one shown while it pauses.
  String? shownDevice;
  String shownPlayer = '';
  void showRemote() {
    final features = c.read(deviceFeaturesProvider.notifier);
    final all = {
      for (final e in c.read(peerMediaProvider).entries)
        if (features.isOn(DeviceFeature.mediaControls, e.key) && e.value.main != null) e.key: e.value,
    };
    String? device;
    if (all[shownDevice]?.main?.playing ?? false) {
      device = shownDevice;
    } else {
      device = all.entries.where((e) => e.value.main!.playing).firstOrNull?.key ??
          (all.containsKey(shownDevice) ? shownDevice : null);
    }
    if (device == null) {
      if (shownDevice != null) _remote.invokeMethod('clear').catchError((Object _) {});
      shownDevice = null;
      return;
    }
    final s = all[device]!;
    final p = s.main!;
    shownDevice = device;
    shownPlayer = p.id;
    _remote.invokeMethod('update', {
      'device': device,
      'title': p.title.isNotEmpty ? p.title : p.name,
      'artist': p.artist,
      'album': p.album,
      'art': p.art,
      'playing': p.playing,
      'canNext': p.canNext,
      'canPrev': p.canPrev,
      'canSeek': p.canSeek,
      'position': s.positionOf(p),
      'length': p.length,
    }).catchError((Object _) {});
  }

  _remote.setMethodCallHandler((call) async {
    final device = shownDevice;
    if (call.method != 'action' || device == null) return;
    final a = (call.arguments as Map).cast<String, dynamic>();
    await sendMediaAction(links, device, shownPlayer, a['action'] as String? ?? '', (a['value'] as num?)?.toDouble() ?? 0);
  });
  c.listen(deviceFeaturesProvider, (_, __) => showRemote());

  links.handle('media.players', (l, p) {
    if (p.body == null) return;
    c.read(peerMediaProvider.notifier).update((m) => {...m, l.peer.name: MediaState.fromJson(p.body!)});
    showRemote();
  });
  links.handle('media.action', (l, p) async {
    final b = p.body ?? const {};
    await _channel.invokeMethod('action', {
      'player': b['player'] as String? ?? '',
      'action': b['action'] as String? ?? '',
      'value': (b['value'] as num?)?.toDouble() ?? 0.0,
    });
  });

  // This phone's media: to each computer as it links up, and on changes.
  final told = <Object>{};
  Future<void> send({bool all = false}) async {
    final targets = links.links.where((l) => l.supports('media.players') && (all || !told.contains(l))).toList();
    if (targets.isEmpty) return;
    Map<String, dynamic> state;
    try {
      state = Map<String, dynamic>.from(await _channel.invokeMethod('state') as Map);
    } catch (_) {
      return;
    }
    for (final l in targets) {
      told.add(l);
      l.send('media.players', body: state).catchError((Object _) {});
    }
  }

  _channel.setMethodCallHandler((call) async {
    if (call.method == 'changed') await send(all: true);
  });
  // Following sessions needs notification access, which may come later.
  var following = false;
  Future<void> follow() async {
    if (following) return;
    try {
      following = await _channel.invokeMethod<bool>('start') ?? false;
    } catch (_) {}
  }

  unawaited(follow());
  Timer.periodic(const Duration(seconds: 3), (_) {
    told.removeWhere((l) => !links.links.contains(l));
    // A computer that went away takes its player with it.
    final media = c.read(peerMediaProvider);
    final gone = media.keys.where((d) => links.byName(d) == null).toList();
    if (gone.isNotEmpty) {
      c.read(peerMediaProvider.notifier).state = {...media}..removeWhere((d, _) => gone.contains(d));
      showRemote();
    }
    unawaited(follow());
    send();
  });
}
