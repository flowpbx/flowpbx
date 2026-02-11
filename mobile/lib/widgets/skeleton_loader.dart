import 'package:flutter/material.dart';
import 'package:shimmer/shimmer.dart';

/// A shimmering placeholder rectangle used to build skeleton loading screens.
class SkeletonBox extends StatelessWidget {
  final double width;
  final double height;
  final double borderRadius;

  const SkeletonBox({
    super.key,
    required this.width,
    required this.height,
    this.borderRadius = 4,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(borderRadius),
      ),
    );
  }
}

/// A shimmering circle placeholder (for avatars).
class SkeletonCircle extends StatelessWidget {
  final double size;

  const SkeletonCircle({super.key, this.size = 40});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        shape: BoxShape.circle,
      ),
    );
  }
}

/// Wraps children in a shimmer animation effect.
class SkeletonShimmer extends StatelessWidget {
  final Widget child;

  const SkeletonShimmer({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Shimmer.fromColors(
      baseColor: isDark
          ? Colors.grey.shade800
          : Colors.grey.shade300,
      highlightColor: isDark
          ? Colors.grey.shade700
          : Colors.grey.shade100,
      child: child,
    );
  }
}

/// Skeleton for a ListTile with a leading circle, title, and subtitle.
class SkeletonListTile extends StatelessWidget {
  final bool showTrailing;

  const SkeletonListTile({super.key, this.showTrailing = true});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          const SkeletonCircle(),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SkeletonBox(width: 140, height: 14),
                const SizedBox(height: 8),
                SkeletonBox(width: 100, height: 12),
              ],
            ),
          ),
          if (showTrailing) ...[
            const SizedBox(width: 16),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                SkeletonBox(width: 40, height: 12),
                const SizedBox(height: 4),
                SkeletonBox(width: 50, height: 10),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

/// Skeleton loading screen for call history — shows shimmer list tiles.
class CallHistorySkeleton extends StatelessWidget {
  const CallHistorySkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return SkeletonShimmer(
      child: ListView.separated(
        physics: const NeverScrollableScrollPhysics(),
        itemCount: 8,
        separatorBuilder: (_, __) => const Divider(height: 1),
        itemBuilder: (_, __) => const SkeletonListTile(),
      ),
    );
  }
}

/// Skeleton loading screen for voicemail — similar to call history.
class VoicemailSkeleton extends StatelessWidget {
  const VoicemailSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return SkeletonShimmer(
      child: ListView.separated(
        physics: const NeverScrollableScrollPhysics(),
        itemCount: 6,
        separatorBuilder: (_, __) => const Divider(height: 1),
        itemBuilder: (_, __) => const SkeletonListTile(),
      ),
    );
  }
}

/// Skeleton loading screen for contacts — includes a search bar placeholder.
class ContactsSkeleton extends StatelessWidget {
  const ContactsSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return SkeletonShimmer(
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: SkeletonBox(
              width: double.infinity,
              height: 48,
              borderRadius: 12,
            ),
          ),
          Expanded(
            child: ListView.separated(
              physics: const NeverScrollableScrollPhysics(),
              itemCount: 10,
              separatorBuilder: (_, __) => const Divider(height: 1),
              itemBuilder: (_, __) =>
                  const SkeletonListTile(showTrailing: false),
            ),
          ),
        ],
      ),
    );
  }
}

/// Skeleton loading screen for settings — shows section headers and list tiles.
class SettingsSkeleton extends StatelessWidget {
  const SettingsSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return SkeletonShimmer(
      child: ListView(
        physics: const NeverScrollableScrollPhysics(),
        children: [
          // Profile section
          _SkeletonSectionHeader(),
          const SkeletonListTile(),
          const Divider(),
          // Call settings section
          _SkeletonSectionHeader(),
          const SkeletonListTile(showTrailing: false),
          const SkeletonListTile(showTrailing: false),
          const Divider(),
          // Connection section
          _SkeletonSectionHeader(),
          const SkeletonListTile(showTrailing: false),
          const SkeletonListTile(showTrailing: false),
          const Divider(),
          // Account section
          _SkeletonSectionHeader(),
          const SkeletonListTile(showTrailing: false),
        ],
      ),
    );
  }
}

class _SkeletonSectionHeader extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: SkeletonBox(width: 80, height: 14),
    );
  }
}
