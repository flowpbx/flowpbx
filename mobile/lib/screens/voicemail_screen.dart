import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/voicemail_entry.dart';
import 'package:flowpbx_mobile/providers/voicemail_player_provider.dart';
import 'package:flowpbx_mobile/providers/voicemail_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/error_banner.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';
import 'package:flowpbx_mobile/widgets/skeleton_loader.dart';

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
        loading: () => const VoicemailSkeleton(),
        error: (error, _) => ErrorBanner(
          error: error,
          fallbackMessage: 'Failed to load voicemails',
          onRetry: () => ref.invalidate(voicemailProvider),
        ),
        data: (entries) {
          if (entries.isEmpty) {
            return RefreshIndicator(
              onRefresh: () => notifier.refresh(),
              child: LayoutBuilder(
                builder: (context, constraints) => SingleChildScrollView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  child: SizedBox(
                    height: constraints.maxHeight,
                    child: Center(
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
                          const SizedBox(height: Dimensions.space16),
                          Text(
                            'No voicemails',
                            style:
                                Theme.of(context).textTheme.bodyLarge?.copyWith(
                                      color: Theme.of(context)
                                          .colorScheme
                                          .onSurfaceVariant,
                                    ),
                          ),
                        ],
                      )
                          .animate()
                          .fadeIn(duration: 400.ms)
                          .scale(
                            begin: const Offset(0.95, 0.95),
                            duration: 400.ms,
                          ),
                    ),
                  ),
                ),
              ),
            );
          }
          return RefreshIndicator(
            onRefresh: () => notifier.refresh(),
            child: ListView.builder(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.symmetric(
                horizontal: Dimensions.space12,
                vertical: Dimensions.space8,
              ),
              itemCount: entries.length,
              itemBuilder: (context, index) {
                return _VoicemailCard(entry: entries[index]);
              },
            ),
          );
        },
      ),
    );
  }
}

/// Card-based voicemail tile with inline player that slides in.
class _VoicemailCard extends ConsumerWidget {
  final VoicemailEntry entry;

  const _VoicemailCard({required this.entry});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
    final isUnread = entry.isUnread;
    final playerState = ref.watch(voicemailPlayerProvider);
    final isActive = playerState.currentId == entry.id;

    return Card(
      margin: const EdgeInsets.symmetric(vertical: Dimensions.space4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            onTap: () => _onTap(ref),
            leading: Stack(
              clipBehavior: Clip.none,
              children: [
                GradientAvatar(
                  name: entry.callerName,
                  radius: Dimensions.avatarRadiusMedium,
                ),
                if (isUnread)
                  Positioned(
                    right: -2,
                    top: -2,
                    child: Container(
                      width: 10,
                      height: 10,
                      decoration: BoxDecoration(
                        color: colorScheme.primary,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: colorScheme.surface,
                          width: 2,
                        ),
                      ),
                    ),
                  ),
              ],
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
                    style: AppTypography.mono(
                      fontSize: 12,
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(width: Dimensions.space8),
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
                  style: AppTypography.mono(
                    fontSize: 12,
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
              ],
            ),
          ),
          if (isActive)
            _PlayerControls(entry: entry)
                .animate()
                .fadeIn(duration: 200.ms)
                .slideY(begin: -0.2, end: 0, duration: 200.ms),
        ],
      ),
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

/// Inline playback controls shown beneath the active voicemail card.
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

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        Dimensions.space16,
        0,
        Dimensions.space16,
        Dimensions.space12,
      ),
      child: Row(
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
            style: AppTypography.mono(
              fontSize: 11,
              color: colorScheme.onSurfaceVariant,
            ),
          ),

          const SizedBox(width: Dimensions.space8),

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
                style: AppTypography.mono(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: colorScheme.primary,
                ),
              ),
            ),
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
