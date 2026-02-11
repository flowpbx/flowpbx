import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_history_entry.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';

/// Fetches paginated call history from the PBX API. Refreshable via
/// ref.invalidate().
final callHistoryProvider =
    FutureProvider<List<CallHistoryEntry>>((ref) async {
  final api = ref.watch(apiServiceProvider);
  final data = await api.getCallHistory(limit: 100, offset: 0);

  final items = data['items'] as List<dynamic>? ?? [];
  return items
      .map((e) => CallHistoryEntry.fromJson(e as Map<String, dynamic>))
      .toList();
});
