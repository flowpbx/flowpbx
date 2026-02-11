import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/connectivity_provider.dart';

/// Wraps the app content and shows a persistent banner at the top when the
/// device has no network connectivity. Automatically hides when connectivity
/// is restored. Adjusts MediaQuery padding so the child scaffold does not
/// double-pad the top safe area.
class OfflineBannerWrapper extends ConsumerWidget {
  final Widget child;

  const OfflineBannerWrapper({super.key, required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isOffline = ref.watch(isOfflineProvider).valueOrNull ?? false;

    if (!isOffline) return child;

    final colorScheme = Theme.of(context).colorScheme;
    final mediaQuery = MediaQuery.of(context);
    final topPadding = mediaQuery.padding.top;

    return Column(
      children: [
        Container(
          width: double.infinity,
          padding: EdgeInsets.only(
            left: 16,
            right: 16,
            top: topPadding + 10,
            bottom: 10,
          ),
          color: colorScheme.errorContainer,
          child: Row(
            children: [
              Icon(
                Icons.cloud_off,
                size: 18,
                color: colorScheme.onErrorContainer,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'No internet connection',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: colorScheme.onErrorContainer,
                      ),
                ),
              ),
            ],
          ),
        )
            .animate()
            .slideY(
              begin: -1,
              end: 0,
              duration: 300.ms,
              curve: Curves.easeOutCubic,
            )
            .fadeIn(duration: 300.ms),
        Expanded(
          child: MediaQuery(
            data: mediaQuery.copyWith(
              padding: mediaQuery.padding.copyWith(top: 0),
            ),
            child: child,
          ),
        ),
      ],
    );
  }
}
