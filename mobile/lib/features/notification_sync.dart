/// notification_sync.dart — mirrors the phone's notifications to paired
/// computers, and carries dismissals, actions and replies back.
///
/// Packet types (must match desktop/notifications_app.go):
///   notification          phone → desktop  {id, app, appId, title, text, time,
///                                           actions: [label], replyable, silent, icon?}
///   notification.removed  phone → desktop  {id}
///   notification.dismiss  desktop → phone  {id}
///   notification.action   desktop → phone  {id, index}
///   notification.reply    desktop → phone  {id, text}
library;

import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../services/link/link.dart';
import '../providers/device_features_provider.dart';
import '../providers/link_provider.dart';
import '../providers/trust_provider.dart';

const _channel = MethodChannel('palt/notifications');
const _enabledKey = 'palt_notif_sync';
const _mutedKey = 'palt_notif_muted';
const _appsKey = 'palt_notif_apps';

/// Settings for notification sync, persisted in SharedPreferences.
class NotificationSyncSettings {
  final bool enabled;
  final Set<String> muted; // package names not forwarded
  final Map<String, String> apps; // package → name, every app seen so far

  const NotificationSyncSettings(this.enabled, this.muted, this.apps);

  static NotificationSyncSettings load(SharedPreferences p) {
    Map<String, String> apps = {};
    try {
      apps = Map<String, String>.from(jsonDecode(p.getString(_appsKey) ?? '{}') as Map);
    } catch (_) {}
    return NotificationSyncSettings(
      p.getBool(_enabledKey) ?? true,
      (p.getStringList(_mutedKey) ?? const []).toSet(),
      apps,
    );
  }
}

final notificationSyncProvider =
    StateNotifierProvider<NotificationSyncNotifier, NotificationSyncSettings>((ref) {
  return NotificationSyncNotifier(ref.read(sharedPreferencesProvider));
});

class NotificationSyncNotifier extends StateNotifier<NotificationSyncSettings> {
  final SharedPreferences _prefs;
  NotificationSyncNotifier(this._prefs) : super(NotificationSyncSettings.load(_prefs));

  void setEnabled(bool on) {
    _prefs.setBool(_enabledKey, on);
    state = NotificationSyncSettings(on, state.muted, state.apps);
  }

  void setMuted(String appId, bool muted) {
    final next = {...state.muted};
    muted ? next.add(appId) : next.remove(appId);
    _prefs.setStringList(_mutedKey, next.toList());
    state = NotificationSyncSettings(state.enabled, next, state.apps);
  }

  void _seen(String appId, String name) {
    if (state.apps[appId] == name) return;
    final apps = {...state.apps, appId: name};
    _prefs.setString(_appsKey, jsonEncode(apps));
    state = NotificationSyncSettings(state.enabled, state.muted, apps);
  }
}

/// Whether Android granted PALT notification access.
Future<bool> hasNotificationAccess() async {
  try {
    return await _channel.invokeMethod<bool>('hasAccess') ?? false;
  } catch (_) {
    return false;
  }
}

Future<void> openNotificationAccessSettings() async {
  try {
    await _channel.invokeMethod('openAccessSettings');
  } catch (_) {}
}

void setupNotificationSync(ProviderContainer c) {
  final links = c.read(linkManagerProvider);
  bool toDevice(Link l) => c.read(deviceFeaturesProvider.notifier).isOn(DeviceFeature.notifications, l.peer.name);

  _channel.setMethodCallHandler((call) async {
    final m = Map<String, dynamic>.from(call.arguments as Map);
    switch (call.method) {
      case 'posted':
        final appId = m['appId'] as String? ?? '';
        final settings = c.read(notificationSyncProvider.notifier);
        settings._seen(appId, m['app'] as String? ?? appId);
        final s = c.read(notificationSyncProvider);
        if (!s.enabled || s.muted.contains(appId)) return;
        links.broadcast('notification', body: m, only: toDevice);
      case 'removed':
        if (c.read(notificationSyncProvider).enabled) links.broadcast('notification.removed', body: m, only: toDevice);
    }
  });

  String id(Map<String, dynamic>? b) => b?['id'] as String? ?? '';
  links.handle('notification.dismiss', (l, p) => _channel.invokeMethod('dismiss', {'id': id(p.body)}));
  links.handle('notification.action',
      (l, p) => _channel.invokeMethod('action', {'id': id(p.body), 'index': p.body?['index'] ?? -1}));
  links.handle('notification.reply',
      (l, p) => _channel.invokeMethod('reply', {'id': id(p.body), 'text': p.body?['text'] ?? ''}));
}
