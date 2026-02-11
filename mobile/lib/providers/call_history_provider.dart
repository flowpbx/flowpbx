import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_history_entry.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/services/call_history_cache_service.dart';

/// Singleton provider for the local call history cache.
final callHistoryCacheProvider = Provider<CallHistoryCacheService>((ref) {
  return CallHistoryCacheService();
});

/// Fetches call history with a cache-first strategy:
/// 1. Return cached entries immediately (if any).
/// 2. Fetch fresh data from the PBX API in the background.
/// 3. Update the cache and provider state with fresh data.
///
/// On network failure the cached data is still returned so the user
/// can browse history offline.
final callHistoryProvider =
    AsyncNotifierProvider<CallHistoryNotifier, List<CallHistoryEntry>>(
  CallHistoryNotifier.new,
);

class CallHistoryNotifier extends AsyncNotifier<List<CallHistoryEntry>> {
  @override
  Future<List<CallHistoryEntry>> build() async {
    final cache = ref.read(callHistoryCacheProvider);

    // Load from cache first for instant display.
    final cached = await cache.getAll();

    // Fire API fetch in the background and update state + cache.
    _fetchFromApi();

    // Return cached data now (may be empty on first launch).
    return cached;
  }

  /// Fetch fresh history from the PBX API, update cache and state.
  Future<void> _fetchFromApi() async {
    try {
      final api = ref.read(apiServiceProvider);
      final cache = ref.read(callHistoryCacheProvider);
      final data = await api.getCallHistory(limit: 100, offset: 0);

      final items = data['items'] as List<dynamic>? ?? [];
      final entries = items
          .map((e) => CallHistoryEntry.fromJson(e as Map<String, dynamic>))
          .toList();

      // Persist to local cache.
      await cache.replaceAll(entries);

      // Update provider state with fresh data.
      state = AsyncData(entries);
    } catch (_) {
      // Network failure — keep showing cached data. If no cached data
      // existed and state is still loading, the empty list from build()
      // will be shown.
    }
  }

  /// Force a refresh from the API (used by pull-to-refresh).
  Future<void> refresh() async {
    await _fetchFromApi();
  }
}
