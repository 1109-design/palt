/// relay_screen.dart — remote transfers through your relay server (Beacon).
///
/// One pairing code links all your devices to a palt-relay server. Paste the
/// code made on another device (or generate one here), and devices that
/// aren't on the same network show up with a RELAY tag.
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/relay_provider.dart';
import '../../services/relay/relay_client.dart';
import '../../services/relay/relay_code.dart';
import '../../theme/app_theme.dart';

Color relayStateColor(RelayStatus? s) => switch (s?.state) {
      RelayState.online => kRelay,
      RelayState.error => kDanger,
      RelayState.connecting => kTextSecondary,
      _ => kTextFaint,
    };

/// App-bar button with a status dot; opens [RelayScreen].
class RelayButton extends ConsumerWidget {
  const RelayButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(relayStatusProvider).valueOrNull;
    final online = status?.state == RelayState.online;
    return IconButton(
      tooltip: 'Relay server',
      onPressed: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => const RelayScreen()),
      ),
      icon: Stack(
        clipBehavior: Clip.none,
        children: [
          Icon(Icons.public_rounded, color: online ? kRelay : kTextSecondary),
          Positioned(
            right: -1,
            top: -1,
            child: Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                color: relayStateColor(status),
                shape: BoxShape.circle,
                border: Border.all(color: kBg, width: 1.5),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class RelayScreen extends ConsumerStatefulWidget {
  const RelayScreen({super.key});

  @override
  ConsumerState<RelayScreen> createState() => _RelayScreenState();
}

class _RelayScreenState extends ConsumerState<RelayScreen> {
  final _codeCtl = TextEditingController();
  final _addrCtl = TextEditingController();
  String? _error;
  bool _reveal = false;
  bool _changing = false;

  @override
  void dispose() {
    _codeCtl.dispose();
    _addrCtl.dispose();
    super.dispose();
  }

  Future<void> _apply(String? code) async {
    setState(() => _error = null);
    try {
      await ref.read(relayCodeProvider.notifier).set(code);
      _codeCtl.clear();
      setState(() => _changing = false);
    } on FormatException catch (e) {
      setState(() => _error = e.message);
    } catch (e) {
      setState(() => _error = e.toString());
    }
  }

  Future<void> _generate() async {
    try {
      final code = RelayCode.generate(_addrCtl.text);
      await _apply(code.toString());
    } on FormatException catch (e) {
      setState(() => _error = e.message);
    }
  }

  Future<void> _paste() async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text?.trim() ?? '';
    if (text.isNotEmpty) setState(() => _codeCtl.text = text);
  }

  String _statusText(String? code, RelayStatus? s) {
    if (code == null) return 'Not set up';
    switch (s?.state) {
      case RelayState.online:
        final n = s!.peers;
        return 'Online · $n other device${n == 1 ? '' : 's'} connected';
      case RelayState.connecting:
        return 'Connecting…';
      case RelayState.error:
        return s!.detail.isEmpty ? 'Cannot reach relay' : s.detail;
      default:
        return 'Starting…';
    }
  }

  @override
  Widget build(BuildContext context) {
    final code = ref.watch(relayCodeProvider);
    final status = ref.watch(relayStatusProvider).valueOrNull;
    final dot = code == null ? kTextFaint : relayStateColor(status);
    final configured = code != null && !_changing;
    RelayCode? parsed;
    if (code != null) {
      try {
        parsed = RelayCode.parse(code);
      } catch (_) {}
    }

    return Scaffold(
      appBar: AppBar(title: Text('Relay server', style: display(fontSize: 19))),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
        children: [
          const Text(
            'Reach your devices when they aren\'t on the same network. Same-network '
            'transfers still go direct. Relayed transfers are end-to-end encrypted and '
            'never stored on the server.',
            style: TextStyle(fontSize: 13, color: kTextSecondary, height: 1.4),
          ),
          const SizedBox(height: 18),

          // Status
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: dot.withValues(alpha: 0.06),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: dot.withValues(alpha: 0.4)),
            ),
            child: Row(
              children: [
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(color: dot, shape: BoxShape.circle),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(_statusText(code, status),
                      style: const TextStyle(fontSize: 13.5, color: kText)),
                ),
                if (parsed != null)
                  Text(parsed.address, style: mono(fontSize: 11, color: kTextFaint)),
              ],
            ),
          ),
          const SizedBox(height: 24),

          if (configured) ...[
            Text('PAIRING CODE', style: overline()),
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.only(left: 14, right: 4),
              decoration: BoxDecoration(
                color: kSurface,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: kHairline),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      _reveal ? code : code.replaceAll(RegExp(r':[^:]+$'), ':${'•' * 16}'),
                      style: mono(fontSize: 11.5, color: kText),
                      maxLines: _reveal ? 3 : 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  IconButton(
                    icon: Icon(_reveal ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                        size: 18, color: kTextFaint),
                    onPressed: () => setState(() => _reveal = !_reveal),
                  ),
                  IconButton(
                    icon: const Icon(Icons.copy_rounded, size: 18, color: kTextFaint),
                    onPressed: () {
                      Clipboard.setData(ClipboardData(text: code));
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('Pairing code copied')),
                      );
                    },
                  ),
                ],
              ),
            ),
            const SizedBox(height: 8),
            const Text(
              'Treat it like a password: anyone with it can send files to your devices.',
              style: TextStyle(fontSize: 12, color: kTextFaint),
            ),
            const SizedBox(height: 24),
            Row(
              children: [
                TextButton(
                  onPressed: () => _apply(null),
                  style: TextButton.styleFrom(foregroundColor: kDanger),
                  child: const Text('Turn off'),
                ),
                const Spacer(),
                TextButton(
                  onPressed: () => setState(() => _changing = true),
                  child: const Text('Use a different code'),
                ),
              ],
            ),
          ] else ...[
            Text('PASTE A CODE FROM ANOTHER DEVICE', style: overline()),
            const SizedBox(height: 8),
            TextField(
              controller: _codeCtl,
              style: mono(fontSize: 12.5, color: kText),
              decoration: InputDecoration(
                hintText: 'palt1:…',
                suffixIcon: IconButton(
                  tooltip: 'Paste',
                  icon: const Icon(Icons.content_paste_rounded, size: 18),
                  onPressed: _paste,
                ),
              ),
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: () => _apply(_codeCtl.text),
                child: const Text('Connect'),
              ),
            ),
            const SizedBox(height: 4),
            const Text(
              'Tip: on the same Wi-Fi, send the code from your computer as a PALT message, '
              'then copy it here.',
              style: TextStyle(fontSize: 12, color: kTextFaint),
            ),
            const SizedBox(height: 28),
            Text('OR CREATE A NEW CODE', style: overline()),
            const SizedBox(height: 8),
            TextField(
              controller: _addrCtl,
              keyboardType: TextInputType.url,
              style: mono(fontSize: 12.5, color: kText),
              decoration: const InputDecoration(hintText: 'relay address, e.g. 203.0.113.7'),
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton(onPressed: _generate, child: const Text('Generate')),
            ),
            if (_changing)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: TextButton(
                  onPressed: () => setState(() => _changing = false),
                  child: const Text('Back'),
                ),
              ),
          ],
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(top: 14),
              child: Text(_error!, style: const TextStyle(color: kDanger, fontSize: 13)),
            ),
        ],
      ),
    );
  }
}
