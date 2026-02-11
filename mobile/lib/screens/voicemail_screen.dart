import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/voicemail_entry.dart';
import 'package:flowpbx_mobile/providers/voicemail_provider.dart';

class VoicemailScreen extends ConsumerWidget {
  const VoicemailScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voicemailAsync = ref.watch(voicemailProvider);
    final notifier = ref.read(voicemailProvider.notifier);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Voicemail'),
      ),
      body: voicemailAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline,
                size: 48,
                color: Theme.of(context).colorScheme.error,
              ),
              const SizedBox(height: 16),
              Text(
                'Failed to load voicemails',
                style: Theme.of(context).textTheme.bodyLarge,
              ),
              const SizedBox(height: 8),
              FilledButton.tonal(
                onPressed: () => ref.invalidate(voicemailProvider),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
        data: (entries) {
          if (entries.isEmpty) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.voicemail,
                    size: 64,
                    color: Theme.of(context)
                        .colorScheme
                        .onSurfaceVariant
                        .withOpacity(0.4),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'No voicemails',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                  ),
                ],
              ),
            );
          }
          return RefreshIndicator(
            onRefresh: () => notifier.refresh(),
            child: ListView.separated(
              itemCount: entries.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (context, index) {
                return _VoicemailTile(entry: entries[index]);
              },
            ),
          );
        },
      ),
    );
  }
}

class _VoicemailTile extends StatelessWidget {
  final VoicemailEntry entry;

  const _VoicemailTile({required this.entry});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final isUnread = entry.isUnread;

    return ListTile(
      leading: CircleAvatar(
        backgroundColor: isUnread
            ? colorScheme.primary.withOpacity(0.12)
            : colorScheme.surfaceContainerHighest,
        child: Icon(
          Icons.voicemail,
          color: isUnread ? colorScheme.primary : colorScheme.onSurfaceVariant,
          size: 20,
        ),
      ),
      title: Text(
        entry.callerName,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: isUnread
            ? const TextStyle(fontWeight: FontWeight.bold)
            : null,
      ),
      subtitle: Row(
        children: [
          if (entry.callerIdNum.isNotEmpty &&
              entry.callerIdNum != entry.callerName) ...[
            Text(
              entry.callerIdNum,
              style: TextStyle(
                color: colorScheme.onSurfaceVariant,
                fontSize: 12,
              ),
            ),
            const SizedBox(width: 8),
          ],
          Text(
            _formatDuration(entry.duration),
            style: TextStyle(
              color: colorScheme.onSurfaceVariant,
              fontSize: 12,
            ),
          ),
        ],
      ),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(
            _formatTime(entry.timestamp),
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 2),
          Text(
            _formatDate(entry.timestamp),
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: colorScheme.onSurfaceVariant.withOpacity(0.7),
                ),
          ),
          if (isUnread) ...[
            const SizedBox(height: 4),
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                color: colorScheme.primary,
                shape: BoxShape.circle,
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _formatDuration(int seconds) {
    if (seconds < 60) return '${seconds}s';
    final minutes = seconds ~/ 60;
    final secs = seconds % 60;
    if (secs == 0) return '${minutes}m';
    return '${minutes}m ${secs}s';
  }

  String _formatTime(DateTime dt) {
    final localDt = dt.toLocal();
    final hour = localDt.hour.toString().padLeft(2, '0');
    final minute = localDt.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }

  String _formatDate(DateTime dt) {
    final localDt = dt.toLocal();
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final entryDate = DateTime(localDt.year, localDt.month, localDt.day);

    if (entryDate == today) return 'Today';
    if (entryDate == today.subtract(const Duration(days: 1))) {
      return 'Yesterday';
    }

    final day = localDt.day.toString().padLeft(2, '0');
    final month = localDt.month.toString().padLeft(2, '0');
    return '$day/$month/${localDt.year}';
  }
}
