import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_history_entry.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/services/call_history_cache_service.dart';

/// Singleton provider for the local call history cache.
final callHistoryCacheProvider = Provider<CallHistoryCacheService>((ref) {
  return CallHistoryCacheService();
});

/// Fetches call history with a cache-first strategy and pagination:
/// 1. Return cached entries immediately (if any).
/// 2. Fetch first page from the PBX API in the background.
/// 3. Support loading more pages on demand (infinite scroll).
///
/// On network failure the cached data is still returned so the user
/// can browse history offline.
final callHistoryProvider =
    AsyncNotifierProvider<CallHistoryNotifier, List<CallHistoryEntry>>(
  CallHistoryNotifier.new,
);

const _pageSize = 50;

class CallHistoryNotifier extends AsyncNotifier<List<CallHistoryEntry>> {
  int _total = 0;
  bool _loadingMore = false;

  /// Whether more pages are available beyond what has been loaded.
  bool get hasMore => (state.valueOrNull?.length ?? 0) < _total;

  /// Whether a loadMore request is currently in progress.
  bool get isLoadingMore => _loadingMore;

  @override
  Future<List<CallHistoryEntry>> build() async {
    _total = 0;
    _loadingMore = false;

    final cache = ref.read(callHistoryCacheProvider);

    // Load from cache first for instant display.
    final cached = await cache.getAll();

    // Fire API fetch in the background and update state + cache.
    _fetchFirstPage();

    // Return cached data now (may be empty on first launch).
    return cached;
  }

  /// Fetch the first page from the PBX API, replace cache and state.
  Future<void> _fetchFirstPage() async {
    try {
      final api = ref.read(apiServiceProvider);
      final cache = ref.read(callHistoryCacheProvider);
      final data = await api.getCallHistory(limit: _pageSize, offset: 0);

      final items = data['items'] as List<dynamic>? ?? [];
      _total = (data['total'] as num?)?.toInt() ?? items.length;

      final entries = items
          .map((e) => CallHistoryEntry.fromJson(e as Map<String, dynamic>))
          .toList();

      // Full refresh — replace entire cache with first page data.
      await cache.replaceAll(entries);

      // Update provider state with fresh data.
      state = AsyncData(entries);
    } catch (_) {
      // Network failure — keep showing cached data.
    }
  }

  /// Load the next page of call history (infinite scroll).
  Future<void> loadMore() async {
    if (_loadingMore || !hasMore) return;

    final current = state.valueOrNull ?? [];
    _loadingMore = true;

    try {
      final api = ref.read(apiServiceProvider);
      final cache = ref.read(callHistoryCacheProvider);
      final data = await api.getCallHistory(
        limit: _pageSize,
        offset: current.length,
      );

      final items = data['items'] as List<dynamic>? ?? [];
      _total = (data['total'] as num?)?.toInt() ?? _total;

      final newEntries = items
          .map((e) => CallHistoryEntry.fromJson(e as Map<String, dynamic>))
          .toList();

      // Append to local cache without wiping existing data.
      await cache.upsertAll(newEntries);

      // Append to state.
      state = AsyncData([...current, ...newEntries]);
    } catch (_) {
      // Network failure — keep existing entries visible.
    } finally {
      _loadingMore = false;
    }
  }

  /// Force a refresh from the API (used by pull-to-refresh).
  /// Resets pagination and reloads the first page.
  Future<void> refresh() async {
    await _fetchFirstPage();
  }
}
