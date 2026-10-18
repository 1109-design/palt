/// storage_path.dart — keeps computers' file paths inside the phone's shared
/// storage.
library;

/// The phone's main shared storage; computers can't go above it.
const kStorageRoot = '/storage/emulated/0';

/// [p] made absolute and tidy, or null when it leaves [kStorageRoot].
String? storagePath(String? p) {
  if (p == null || p.isEmpty) return kStorageRoot;
  final parts = <String>[];
  for (final s in p.split('/')) {
    if (s.isEmpty || s == '.') continue;
    if (s == '..') {
      if (parts.isNotEmpty) parts.removeLast();
      continue;
    }
    parts.add(s);
  }
  final n = '/${parts.join('/')}';
  return n == kStorageRoot || n.startsWith('$kStorageRoot/') ? n : null;
}
