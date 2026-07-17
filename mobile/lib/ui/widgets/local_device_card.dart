/// local_device_card.dart — "this device" identity strip (Beacon).
///
/// Your phone drawn as a beacon: OS tile wrapped in pulsing rings while
/// discovery broadcasts, hostname in display type, address as a mono readout.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../providers/peer_provider.dart';
import '../../theme/app_theme.dart';
import 'beacon_pulse.dart';
import 'dart:io';

class LocalDeviceCard extends ConsumerWidget {
  const LocalDeviceCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ipAsync = ref.watch(localIpProvider);
    final ipAddress = ipAsync.valueOrNull ?? '…';
    final hostname =
        Platform.localHostname.isNotEmpty ? Platform.localHostname : 'palt-android';

    return Container(
      margin: const EdgeInsets.fromLTRB(20, 8, 20, 0),
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 24),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: kHairline),
      ),
      child: Row(
        children: [
          BeaconPulse(
            active: true,
            size: 60,
            child: Container(
              width: 60,
              height: 60,
              decoration: BoxDecoration(
                color: kSurfaceRaised,
                shape: BoxShape.circle,
                border: Border.all(color: kHairline),
              ),
              child: const Icon(Icons.smartphone_rounded, size: 26, color: kAccent),
            ),
          ),
          const SizedBox(width: 22),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('THIS DEVICE', style: overline()),
                const SizedBox(height: 3),
                Text(
                  hostname,
                  style: display(fontSize: 22, fontWeight: FontWeight.w800),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    Container(
                      padding:
                          const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: kBg,
                        borderRadius: BorderRadius.circular(6),
                        border: Border.all(color: kHairline),
                      ),
                      child: Text('$ipAddress:9876', style: mono()),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          // Broadcast lamp
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(
              color: kAccent.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(999),
              border: Border.all(color: kAccent.withValues(alpha: 0.35)),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 7,
                  height: 7,
                  decoration: BoxDecoration(
                    color: kAccent,
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: kAccent.withValues(alpha: 0.5),
                        blurRadius: 6,
                        spreadRadius: 1,
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 6),
                const Text(
                  'On air',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: kAccent,
                    letterSpacing: 0.3,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
