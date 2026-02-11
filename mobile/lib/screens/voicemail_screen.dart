import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/voicemail_entry.dart';
import 'package:flowpbx_mobile/providers/voicemail_player_provider.dart';
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

class _VoicemailTile extends ConsumerWidget {
  final VoicemailEntry entry;

  const _VoicemailTile({required this.entry});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
    final isUnread = entry.isUnread;
    final playerState = ref.watch(voicemailPlayerProvider);
    final isActive = playerState.currentId == entry.id;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        ListTile(
          onTap: () => _onTap(ref),
          leading: CircleAvatar(
            backgroundColor: isUnread
                ? colorScheme.primary.withOpacity(0.12)
                : colorScheme.surfaceContainerHighest,
            child: Icon(
              isActive && playerState.isPlaying
                  ? Icons.pause
                  : Icons.voicemail,
              color:
                  isUnread ? colorScheme.primary : colorScheme.onSurfaceVariant,
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
        ),
        if (isActive) _PlayerControls(entry: entry),
      ],
    );
  }

  void _onTap(WidgetRef ref) {
    final player = ref.read(voicemailPlayerProvider.notifier);
    player.play(entry.id);

    // Mark as read when tapped for playback.
    if (entry.isUnread) {
      ref.read(voicemailProvider.notifier).markRead(entry.id);
    }
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

/// Inline playback controls shown beneath the active voicemail tile.
class _PlayerControls extends ConsumerWidget {
  final VoicemailEntry entry;

  const _PlayerControls({required this.entry});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
    final playerState = ref.watch(voicemailPlayerProvider);
    final player = ref.read(voicemailPlayerProvider.notifier);

    final position = playerState.position;
    final duration =
        playerState.duration > Duration.zero ? playerState.duration : null;

    return Container(
      color: colorScheme.surfaceContainerHighest.withOpacity(0.5),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              // Play/pause button.
              SizedBox(
                width: 40,
                height: 40,
                child: playerState.isLoading
                    ? const Padding(
                        padding: EdgeInsets.all(10),
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : IconButton(
                        icon: Icon(
                          playerState.isPlaying
                              ? Icons.pause_rounded
                              : Icons.play_arrow_rounded,
                          size: 24,
                        ),
                        padding: EdgeInsets.zero,
                        onPressed: () => player.play(entry.id),
                      ),
              ),

              // Seek slider.
              Expanded(
                child: SliderTheme(
                  data: SliderTheme.of(context).copyWith(
                    trackHeight: 3,
                    thumbShape:
                        const RoundSliderThumbShape(enabledThumbRadius: 6),
                    overlayShape:
                        const RoundSliderOverlayShape(overlayRadius: 14),
                  ),
                  child: Slider(
                    value: duration != null
                        ? position.inMilliseconds
                            .clamp(0, duration.inMilliseconds)
                            .toDouble()
                        : 0,
                    max: duration?.inMilliseconds.toDouble() ?? 1,
                    onChanged: duration != null
                        ? (value) {
                            player
                                .seek(Duration(milliseconds: value.toInt()));
                          }
                        : null,
                  ),
                ),
              ),

              // Position / duration label.
              Text(
                '${_formatPos(position)} / ${_formatPos(duration ?? Duration.zero)}',
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                      fontFeatures: [const FontFeature.tabularFigures()],
                    ),
              ),

              const SizedBox(width: 8),

              // Speed button.
              SizedBox(
                width: 44,
                height: 32,
                child: TextButton(
                  style: TextButton.styleFrom(
                    padding: EdgeInsets.zero,
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  onPressed: () => player.cycleSpeed(),
                  child: Text(
                    '${playerState.speed}x',
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: colorScheme.primary,
                        ),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  String _formatPos(Duration d) {
    final minutes = d.inMinutes;
    final seconds = d.inSeconds % 60;
    return '${minutes.toString().padLeft(1, '0')}:${seconds.toString().padLeft(2, '0')}';
  }
}
