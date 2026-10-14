import 'dart:convert';
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../models/history_entry.dart';

final historyProvider = StateNotifierProvider<HistoryNotifier, List<HistoryEntry>>((ref) {
  return HistoryNotifier();
});

class HistoryNotifier extends StateNotifier<List<HistoryEntry>> {
  HistoryNotifier() : super([]) {
    _loadHistory();
  }

  static const _key = 'transfer_history';

  Future<void> _loadHistory() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final historyJson = prefs.getStringList(_key);
      if (historyJson != null) {
        state = historyJson
            .map((e) => HistoryEntry.fromJson(jsonDecode(e)))
            .toList();
      }
    } catch (e) {
      print('[HistoryNotifier] Error loading history: $e');
    }
  }

  Future<void> addEntry(HistoryEntry entry) async {
    // Prepend new entry
    state = [entry, ...state];
    
    // Keep last 500 items for mobile to save space
    if (state.length > 500) {
      state = state.sublist(0, 500);
    }
    
    await _saveHistory();
  }

  /// Removes one thread item. [entry] may be a grouped batch from the device
  /// page (id "direction:transferId"), which covers several stored entries.
  /// Files on disk are kept; previews made for sent pictures are deleted.
  Future<void> removeEntry(HistoryEntry entry) async {
    bool matches(HistoryEntry e) {
      final colon = entry.id.indexOf(':');
      if (colon > 0 && !entry.isText) {
        final dir = entry.id.substring(0, colon);
        final tid = entry.id.substring(colon + 1);
        return e.direction == dir && e.id.startsWith('${tid}_');
      }
      return e.id == entry.id && e.direction == entry.direction;
    }

    final removed = state.where(matches).toList();
    if (removed.isEmpty) return;
    state = state.where((e) => !matches(e)).toList();
    await _saveHistory();
    for (final e in removed) {
      for (final f in e.files) {
        if (f.preview != null) {
          try {
            await File(f.preview!).delete();
          } catch (_) {}
        }
      }
    }
  }

  Future<void> clearHistory() async {
    state = [];
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_key);
  }

  Future<void> _saveHistory() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final historyJson = state.map((e) => jsonEncode(e.toJson())).toList();
      await prefs.setStringList(_key, historyJson);
    } catch (e) {
      print('[HistoryNotifier] Error saving history: $e');
    }
  }
}
