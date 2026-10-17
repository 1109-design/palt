/// route_badge.dart — how a peer or transfer travels (Beacon).
///
/// USB: over the cable (USB tethering). WI-FI: same local network, device
/// to device. RELAY: through the palt-relay server, end-to-end encrypted.
library;

import 'package:flutter/material.dart';

import '../../services/usb_path.dart';
import '../../theme/app_theme.dart';

/// 'usb', 'wifi' or 'relay' for a route and, on direct routes, the peer's
/// IP ('direct' when the IP isn't known).
String pathOf(String? route, [String? ip]) => route == 'relay'
    ? 'relay'
    : ip == null
        ? 'direct'
        : isUsbIp(ip)
            ? 'usb'
            : 'wifi';

Color routeColor(String? route, [String? ip]) => switch (pathOf(route, ip)) {
      'relay' => kRelay,
      'usb' => kOk,
      _ => kTextSecondary,
    };

String routeLabel(String? route, [String? ip]) => switch (pathOf(route, ip)) {
      'relay' => 'via relay',
      'usb' => 'USB cable',
      'wifi' => 'same Wi-Fi',
      _ => 'direct',
    };

class RouteBadge extends StatelessWidget {
  final String? route;

  /// The peer's address on direct routes; tells a USB cable from Wi-Fi.
  final String? ip;
  const RouteBadge({super.key, required this.route, this.ip});

  @override
  Widget build(BuildContext context) => ValueListenableBuilder(
        valueListenable: usbPrefixes,
        builder: (context, _, __) {
          final path = pathOf(route, ip);
          final color = routeColor(route, ip);
          return Tooltip(
            message: switch (path) {
              'relay' => 'Not on this network — goes through your relay server, end-to-end encrypted',
              'usb' => 'Over the USB cable — works without Wi-Fi or data',
              'wifi' => 'Same Wi-Fi — sent device to device',
              _ => 'Device to device, without the relay',
            },
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
                    switch (path) { 'relay' => 'RELAY', 'usb' => 'USB', 'wifi' => 'WI-FI', _ => 'DIRECT' },
                    style: mono(fontSize: 9.5, fontWeight: FontWeight.w700, color: color),
                  ),
                ],
              ),
            ),
          );
        },
      );
}
