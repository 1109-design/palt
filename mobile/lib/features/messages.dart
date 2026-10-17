/// messages.dart — contacts and SMS for paired computers.
///
/// Computers ask for conversations and threads, send texts through this
/// phone, and hear about new texts as they arrive (Messages.kt). Contacts
/// go over in parts, again only when they change. No MMS.
///
/// Packet types (must match desktop/messages_app.go). Requests carry an id
/// and are answered by "<type>.reply" with the same id.
///   sms.conversations  computer → phone  {limit} → {threads: [message + unread, name]}
///   sms.thread         computer → phone  {threadId, before, limit} → {messages}
///   sms.send           computer → phone  {address, body} → {ok, error}
///   sms.received       phone → computer  message {id, threadId, address, body, date, type, read, name}
///   sms.changed        phone → computer  {} the SMS store changed (sent, read, …)
///   contacts           computer → phone  {version} → {version, parts} then
///   contacts.part      phone → computer  {version, part, parts, contacts: [{id, name, numbers, photo}]}
///   contacts.changed   phone → computer  {} ask again
library;

import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/link_provider.dart';
import '../services/link/link.dart';

const _channel = MethodChannel('palt/messages');
const _contactsPerPart = 150;

/// Wires contacts and SMS into the running core. Call once from startCore.
void setupMessages(ProviderContainer c) {
  final links = c.read(linkManagerProvider);

  // Answers run off the link's read loop: queries can take a moment.
  void answer(String type, Future<Map<String, dynamic>> Function(Map<String, dynamic> body) work) {
    links.handle(type, (l, p) {
      unawaited(() async {
        Map<String, dynamic> reply;
        try {
          reply = await work(p.body ?? const {});
        } on PlatformException catch (e) {
          reply = {'error': e.message ?? e.code};
        } catch (e) {
          reply = {'error': e.toString()};
        }
        await l.send('$type.reply', id: p.id, body: reply).catchError((Object _) {});
      }());
    });
  }

  List<Map<String, dynamic>> maps(Object? list) =>
      ((list as List?) ?? const []).map((m) => Map<String, dynamic>.from(m as Map)).toList();

  answer('sms.conversations', (b) async {
    unawaited(_channel.invokeMethod('start'));
    final threads = await _channel.invokeMethod('conversations', {'limit': (b['limit'] as num?)?.toInt() ?? 100});
    return {'threads': maps(threads)};
  });
  answer('sms.thread', (b) async {
    final msgs = await _channel.invokeMethod('thread', {
      'threadId': (b['threadId'] as num?)?.toInt() ?? 0,
      'before': (b['before'] as num?)?.toInt() ?? 0,
      'limit': (b['limit'] as num?)?.toInt() ?? 50,
    });
    return {'messages': maps(msgs)};
  });
  answer('sms.send', (b) async {
    final address = b['address'] as String? ?? '';
    final body = b['body'] as String? ?? '';
    if (address.isEmpty || body.isEmpty) return {'ok': false, 'error': 'Nothing to send'};
    await _channel.invokeMethod('send', {'address': address, 'body': body});
    return {'ok': true};
  });

  links.handle('contacts', (l, p) {
    unawaited(() async {
      Map<String, dynamic> all;
      try {
        all = Map<String, dynamic>.from(await _channel.invokeMethod('contacts') as Map);
      } catch (e) {
        await l.send('contacts.reply', id: p.id, body: {'error': e.toString()}).catchError((Object _) {});
        return;
      }
      final version = all['version'] as String? ?? '';
      final list = maps(all['contacts']);
      if (version.isNotEmpty && version == p.body?['version']) {
        await l.send('contacts.reply', id: p.id, body: {'version': version, 'parts': 0});
        return;
      }
      final parts = (list.length / _contactsPerPart).ceil();
      await l.send('contacts.reply', id: p.id, body: {'version': version, 'parts': parts});
      for (var i = 0; i < parts; i++) {
        await l.send('contacts.part', body: {
          'version': version,
          'part': i,
          'parts': parts,
          'contacts': list.skip(i * _contactsPerPart).take(_contactsPerPart).toList(),
        });
      }
    }());
  });

  _channel.setMethodCallHandler((call) async {
    switch (call.method) {
      case 'received':
        links.broadcast('sms.received', body: Map<String, dynamic>.from(call.arguments as Map));
      case 'changed':
        links.broadcast(call.arguments == 'contacts' ? 'contacts.changed' : 'sms.changed', body: {});
    }
  });
  // Follow new texts once SMS permission is there (it may come later).
  Timer.periodic(const Duration(seconds: 30), (_) => _channel.invokeMethod('start').catchError((Object _) => false));
  unawaited(_channel.invokeMethod('start').catchError((Object _) => false));
}

/// Whether [l] can use this phone's messages.
bool supportsMessages(Link l) => l.supports('sms.conversations');
