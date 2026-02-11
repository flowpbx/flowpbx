import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_history_entry.dart';
import 'package:flowpbx_mobile/providers/call_history_provider.dart';
import 'package:go_router/go_router.dart';

class CallHistoryScreen extends ConsumerWidget {
  const CallHistoryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final historyAsync = ref.watch(callHistoryProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Call History'),
      ),
      body: historyAsync.when(
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
                'Failed to load call history',
                style: Theme.of(context).textTheme.bodyLarge,
              ),
              const SizedBox(height: 8),
              FilledButton.tonal(
                onPressed: () => ref.invalidate(callHistoryProvider),
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
                    Icons.history,
                    size: 64,
                    color: Theme.of(context)
                        .colorScheme
                        .onSurfaceVariant
                        .withOpacity(0.4),
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'No call history',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                  ),
                ],
              ),
            );
          }
          return RefreshIndicator(
            onRefresh: () =>
                ref.read(callHistoryProvider.notifier).refresh(),
            child: ListView.separated(
              itemCount: entries.length,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (context, index) {
                return _CallHistoryTile(entry: entries[index]);
              },
            ),
          );
        },
      ),
    );
  }
}

class _CallHistoryTile extends StatelessWidget {
  final CallHistoryEntry entry;

  const _CallHistoryTile({required this.entry});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final icon = _directionIcon();
    final iconColor = _iconColor(colorScheme);

    return ListTile(
      leading: CircleAvatar(
        backgroundColor: iconColor.withOpacity(0.12),
        child: Icon(icon, color: iconColor, size: 20),
      ),
      title: Text(
        entry.remoteName.isNotEmpty ? entry.remoteName : 'Unknown',
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Row(
        children: [
          Icon(_directionArrow(), size: 14, color: iconColor),
          const SizedBox(width: 4),
          Text(
            _directionLabel(),
            style: TextStyle(color: colorScheme.onSurfaceVariant, fontSize: 12),
          ),
          if (entry.duration != null && entry.duration! > 0) ...[
            const SizedBox(width: 8),
            Text(
              _formatDuration(entry.duration!),
              style:
                  TextStyle(color: colorScheme.onSurfaceVariant, fontSize: 12),
            ),
          ],
        ],
      ),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(
            _formatTime(context, entry.startTime),
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 2),
          Text(
            _formatDate(entry.startTime),
            style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: colorScheme.onSurfaceVariant.withOpacity(0.7),
                ),
          ),
        ],
      ),
      onTap: () {
        context.go('/dialpad?number=${entry.remoteNumber}');
      },
    );
  }

  IconData _directionIcon() {
    if (entry.isMissed) return Icons.call_missed;
    if (entry.isOutbound) return Icons.call_made;
    return Icons.call_received;
  }

  IconData _directionArrow() {
    if (entry.isOutbound) return Icons.arrow_outward;
    return Icons.arrow_downward;
  }

  Color _iconColor(ColorScheme colorScheme) {
    if (entry.isMissed) return colorScheme.error;
    if (entry.isOutbound) return Colors.blue;
    return Colors.green;
  }

  String _directionLabel() {
    if (entry.isMissed) return 'Missed';
    switch (entry.direction) {
      case 'outbound':
        return 'Outgoing';
      case 'inbound':
        return 'Incoming';
      case 'internal':
        return 'Internal';
      default:
        return entry.direction;
    }
  }

  String _formatDuration(int seconds) {
    if (seconds < 60) return '${seconds}s';
    final minutes = seconds ~/ 60;
    final secs = seconds % 60;
    if (secs == 0) return '${minutes}m';
    return '${minutes}m ${secs}s';
  }

  String _formatTime(BuildContext context, DateTime dt) {
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
