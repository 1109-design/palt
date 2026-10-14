/// route_badge.dart — how a peer or transfer travels (Beacon).
///
/// DIRECT: same local network, device to device. RELAY: through the
/// palt-relay server, end-to-end encrypted. Direct is always preferred.
library;

import 'package:flutter/material.dart';

import '../../theme/app_theme.dart';

Color routeColor(String? route) => route == 'relay' ? kRelay : kTextSecondary;

String routeLabel(String? route) => route == 'relay' ? 'via relay' : 'direct';

class RouteBadge extends StatelessWidget {
  final String? route;
  const RouteBadge({super.key, required this.route});

  @override
  Widget build(BuildContext context) {
    final relay = route == 'relay';
    final color = routeColor(route);
    return Tooltip(
      message: relay
          ? 'Not on this network — goes through your relay server, end-to-end encrypted'
          : 'Same network — sent device to device',
      child: Container(
        height: 18,
        padding: const EdgeInsets.symmetric(horizontal: 7),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.08),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(color: color.withValues(alpha: 0.45)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 5,
              height: 5,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            ),
            const SizedBox(width: 5),
            Text(
              relay ? 'RELAY' : 'DIRECT',
              style: mono(fontSize: 9.5, fontWeight: FontWeight.w700, color: color),
            ),
          ],
        ),
      ),
    );
  }
}
