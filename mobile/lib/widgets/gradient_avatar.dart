import 'package:flutter/material.dart';
import 'package:flowpbx_mobile/theme/typography.dart';

/// A [CircleAvatar] that picks a gradient background based on the hash of
/// the provided [name]. Shows the user's initials in white.
class GradientAvatar extends StatelessWidget {
  final String name;
  final double radius;

  const GradientAvatar({
    super.key,
    required this.name,
    this.radius = 20,
  });

  static const _gradientPairs = <(Color, Color)>[
    (Color(0xFF2563EB), Color(0xFF7C3AED)), // blue → violet
    (Color(0xFF0891B2), Color(0xFF2563EB)), // cyan → blue
    (Color(0xFF059669), Color(0xFF0891B2)), // emerald → cyan
    (Color(0xFFD97706), Color(0xFFDC2626)), // amber → red
    (Color(0xFF7C3AED), Color(0xFFEC4899)), // violet → pink
    (Color(0xFF0D9488), Color(0xFF059669)), // teal → emerald
  ];

  (Color, Color) _gradientForName(String name) {
    final hash = name.hashCode.abs();
    return _gradientPairs[hash % _gradientPairs.length];
  }

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }

  @override
  Widget build(BuildContext context) {
    final (start, end) = _gradientForName(name);

    return Container(
      width: radius * 2,
      height: radius * 2,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [start, end],
        ),
      ),
      alignment: Alignment.center,
      child: Text(
        _initials(name),
        style: AppTypography.mono(
          fontSize: radius * 0.8,
          fontWeight: FontWeight.w600,
          color: Colors.white,
        ),
      ),
    );
  }
}
