import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_history_entry.dart';
import 'package:flowpbx_mobile/providers/call_history_provider.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/missed_call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/app_error.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/error_banner.dart';
import 'package:flowpbx_mobile/widgets/skeleton_loader.dart';

class CallHistoryScreen extends ConsumerStatefulWidget {
  const CallHistoryScreen({super.key});

  @override
  ConsumerState<CallHistoryScreen> createState() => _CallHistoryScreenState();
}

class _CallHistoryScreenState extends ConsumerState<CallHistoryScreen> {
  final _scrollController = ScrollController();
  bool _firstLoad = true;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    // Mark all missed calls as seen so the badge clears.
    // Deferred to avoid modifying providers during widget tree build.
    Future(() {
      ref.read(lastSeenMissedCallProvider.notifier).markAllSeen();
    });
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final maxScroll = _scrollController.position.maxScrollExtent;
    final currentScroll = _scrollController.position.pixels;
    // Trigger load when within 200px of the bottom.
    if (maxScroll - currentScroll <= 200) {
      final notifier = ref.read(callHistoryProvider.notifier);
      if (notifier.hasMore && !notifier.isLoadingMore) {
        notifier.loadMore();
      }
    }
  }

  String _dateGroup(DateTime dt) {
    final localDt = dt.toLocal();
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final entryDate = DateTime(localDt.year, localDt.month, localDt.day);

    if (entryDate == today) return 'Today';
    if (entryDate == today.subtract(const Duration(days: 1))) {
      return 'Yesterday';
    }
    final daysAgo = today.difference(entryDate).inDays;
    if (daysAgo < 7) return 'This Week';
    return 'Older';
  }

  @override
  Widget build(BuildContext context) {
    final historyAsync = ref.watch(callHistoryProvider);
    final notifier = ref.read(callHistoryProvider.notifier);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Recents'),
      ),
      body: historyAsync.when(
        loading: () => const CallHistorySkeleton(),
        error: (error, _) => ErrorBanner(
          error: error,
          fallbackMessage: 'Failed to load call history',
          onRetry: () => ref.invalidate(callHistoryProvider),
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
                  const SizedBox(height: Dimensions.space16),
                  Text(
                    'No call history',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                  ),
                ],
              )
                  .animate()
                  .fadeIn(duration: 400.ms)
                  .scale(begin: const Offset(0.95, 0.95), duration: 400.ms),
            );
          }

          // Build grouped list with section headers.
          final items = <_ListItem>[];
          String? lastGroup;
          for (final entry in entries) {
            final group = _dateGroup(entry.startTime);
            if (group != lastGroup) {
              items.add(_ListItem.header(group));
              lastGroup = group;
            }
            items.add(_ListItem.entry(entry));
          }
          if (notifier.hasMore) {
            items.add(_ListItem.loading());
          }

          final animate = _firstLoad;
          if (_firstLoad) _firstLoad = false;

          return RefreshIndicator(
            onRefresh: () => notifier.refresh(),
            child: ListView.builder(
              controller: _scrollController,
              itemCount: items.length,
              itemBuilder: (context, index) {
                final item = items[index];
                if (item.isHeader) {
                  return Padding(
                    padding: const EdgeInsets.fromLTRB(
                      Dimensions.space16,
                      Dimensions.space16,
                      Dimensions.space16,
                      Dimensions.space4,
                    ),
                    child: Text(
                      item.headerTitle!,
                      style:
                          Theme.of(context).textTheme.labelLarge?.copyWith(
                                color: Theme.of(context).colorScheme.primary,
                                fontWeight: FontWeight.w600,
                              ),
                    ),
                  );
                }
                if (item.isLoading) {
                  return const Padding(
                    padding: EdgeInsets.symmetric(vertical: 16),
                    child: Center(child: CircularProgressIndicator()),
                  );
                }
                Widget tile = _CallHistoryTile(entry: item.entry!);
                if (animate && index < 20) {
                  tile = tile
                      .animate()
                      .fadeIn(
                        duration: 200.ms,
                        delay: (30 * index).ms,
                      )
                      .slideX(
                        begin: 0.05,
                        end: 0,
                        duration: 200.ms,
                        delay: (30 * index).ms,
                        curve: Curves.easeOut,
                      );
                }
                return tile;
              },
            ),
          );
        },
      ),
    );
  }
}

/// Union type for list items: section header, entry, or loading indicator.
class _ListItem {
  final String? headerTitle;
  final CallHistoryEntry? entry;
  final bool isLoading;

  _ListItem._({this.headerTitle, this.entry, this.isLoading = false});

  factory _ListItem.header(String title) =>
      _ListItem._(headerTitle: title);
  factory _ListItem.entry(CallHistoryEntry entry) =>
      _ListItem._(entry: entry);
  factory _ListItem.loading() => _ListItem._(isLoading: true);

  bool get isHeader => headerTitle != null;
}

class _CallHistoryTile extends ConsumerWidget {
  final CallHistoryEntry entry;

  const _CallHistoryTile({required this.entry});

  Future<void> _callBack(BuildContext context, WidgetRef ref) async {
    final number = entry.remoteNumber;
    if (number.isEmpty) return;

    final sipService = ref.read(sipServiceProvider);
    if (!sipService.isRegistered) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Not registered — cannot place call')),
      );
      return;
    }

    try {
      await sipService.invite(number);
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Call failed: ${formatError(e)}')),
      );
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
    final icon = _directionIcon();
    final iconColor = _iconColor(colorScheme);
    final callAsync = ref.watch(callStateProvider);
    final hasActiveCall = callAsync.valueOrNull?.isActive ?? false;

    return ListTile(
      leading: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: iconColor.withOpacity(0.12),
          borderRadius: Dimensions.borderRadiusSmall,
        ),
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
            style:
                TextStyle(color: colorScheme.onSurfaceVariant, fontSize: 12),
          ),
          if (entry.duration != null && entry.duration! > 0) ...[
            const SizedBox(width: 8),
            Text(
              _formatDuration(entry.duration!),
              style: TextStyle(
                  color: colorScheme.onSurfaceVariant, fontSize: 12),
            ),
          ],
        ],
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                _formatTime(context, entry.startTime),
                style: AppTypography.mono(
                  fontSize: 12,
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
          const SizedBox(width: 8),
          Icon(
            Icons.call,
            size: 20,
            color: hasActiveCall
                ? colorScheme.onSurfaceVariant.withOpacity(0.3)
                : ColorTokens.callGreen,
          ),
        ],
      ),
      onTap: hasActiveCall ? null : () => _callBack(context, ref),
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
    if (entry.isOutbound) return colorScheme.primary;
    return ColorTokens.callGreen;
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
