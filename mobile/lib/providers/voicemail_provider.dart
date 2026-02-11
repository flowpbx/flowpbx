import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/voicemail_entry.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';

/// Count of unread voicemail messages (drives the AppBar badge).
final unreadVoicemailCountProvider = Provider<int>((ref) {
  final voicemails = ref.watch(voicemailProvider).valueOrNull ?? [];
  return voicemails.where((e) => e.isUnread).length;
});

/// Fetches voicemail messages from the PBX API.
final voicemailProvider =
    AsyncNotifierProvider<VoicemailNotifier, List<VoicemailEntry>>(
  VoicemailNotifier.new,
);

class VoicemailNotifier extends AsyncNotifier<List<VoicemailEntry>> {
  @override
  Future<List<VoicemailEntry>> build() async {
    return _fetch();
  }

  Future<List<VoicemailEntry>> _fetch() async {
    final api = ref.read(apiServiceProvider);
    final data = await api.getVoicemails();
    final entries = data
        .map((e) => VoicemailEntry.fromJson(e as Map<String, dynamic>))
        .toList();
    // Sort newest first.
    entries.sort((a, b) => b.timestamp.compareTo(a.timestamp));
    return entries;
  }

  /// Force a refresh from the API (used by pull-to-refresh).
  Future<void> refresh() async {
    state = AsyncData(await _fetch());
  }

  /// Mark a voicemail as read and update local state.
  Future<void> markRead(int id) async {
    final api = ref.read(apiServiceProvider);
    await api.markVoicemailRead(id);
    // Update the entry in state without a full refetch.
    final current = state.valueOrNull ?? [];
    state = AsyncData(current.map((e) {
      if (e.id == id) {
        return VoicemailEntry(
          id: e.id,
          mailboxId: e.mailboxId,
          callerIdName: e.callerIdName,
          callerIdNum: e.callerIdNum,
          timestamp: e.timestamp,
          duration: e.duration,
          read: true,
          readAt: DateTime.now(),
          transcription: e.transcription,
          createdAt: e.createdAt,
        );
      }
      return e;
    }).toList());
  }
}
