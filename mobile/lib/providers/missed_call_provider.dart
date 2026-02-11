import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/call_history_provider.dart';
import 'package:flowpbx_mobile/services/call_history_cache_service.dart';

/// Provides the count of missed inbound calls that the user has not yet seen.
///
/// A missed call is "unseen" if it occurred after the last time the user
/// opened the call history screen. The last-seen timestamp is persisted in
/// the local SQLite cache so the badge survives app restarts.
final missedCallCountProvider = Provider<int>((ref) {
  final history = ref.watch(callHistoryProvider).valueOrNull ?? [];
  final lastSeen = ref.watch(lastSeenMissedCallProvider);

  return history.where((e) {
    return e.isInbound && e.isMissed && e.startTime.isAfter(lastSeen);
  }).length;
});

/// Tracks the DateTime after which missed calls are considered "seen".
/// Persisted to the local cache DB via CallHistoryCacheService.
final lastSeenMissedCallProvider =
    NotifierProvider<LastSeenMissedCallNotifier, DateTime>(
  LastSeenMissedCallNotifier.new,
);

class LastSeenMissedCallNotifier extends Notifier<DateTime> {
  @override
  DateTime build() {
    // Load persisted value on init. Default to epoch (show all missed).
    _loadFromCache();
    return DateTime.fromMillisecondsSinceEpoch(0);
  }

  Future<void> _loadFromCache() async {
    final cache = ref.read(callHistoryCacheProvider);
    final ts = await cache.getLastSeenMissedCall();
    if (ts != null) {
      state = ts;
    }
  }

  /// Mark all current missed calls as seen (user opened call history).
  Future<void> markAllSeen() async {
    final now = DateTime.now().toUtc();
    state = now;
    final cache = ref.read(callHistoryCacheProvider);
    await cache.setLastSeenMissedCall(now);
  }
}
