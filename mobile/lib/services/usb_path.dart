/// usb_path.dart — tells whether a computer is reached over the USB cable
/// (USB tethering) rather than Wi-Fi, for the route shown in the UI.
library;

import 'dart:io';

import 'package:flutter/foundation.dart';

/// The /24 prefixes ("10.242.29.") of this phone's USB tethering
/// interfaces; refreshed on every network change.
final usbPrefixes = ValueNotifier<Set<String>>(const {});

Future<void> refreshUsbPrefixes() async {
  try {
    final ifaces = await NetworkInterface.list(type: InternetAddressType.IPv4);
    usbPrefixes.value = {
      for (final i in ifaces)
        if (i.name.startsWith('rndis') || i.name.startsWith('ncm') || i.name.startsWith('usb'))
          for (final a in i.addresses) a.address.substring(0, a.address.lastIndexOf('.') + 1),
    };
  } catch (_) {}
}

/// True when [ip] is on this phone's USB tethering network.
bool isUsbIp(String? ip) {
  if (ip == null || !ip.contains('.')) return false;
  return usbPrefixes.value.contains(ip.substring(0, ip.lastIndexOf('.') + 1));
}
