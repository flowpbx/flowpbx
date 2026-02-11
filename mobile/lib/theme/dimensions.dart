import 'package:flutter/material.dart';

/// Spacing scale and component dimensions for consistent layout.
abstract final class Dimensions {
  // ── Spacing (4 px base) ──────────────────────────────────────────────
  static const double space2 = 2;
  static const double space4 = 4;
  static const double space8 = 8;
  static const double space12 = 12;
  static const double space16 = 16;
  static const double space20 = 20;
  static const double space24 = 24;
  static const double space32 = 32;
  static const double space40 = 40;
  static const double space48 = 48;

  // ── Border radii ─────────────────────────────────────────────────────
  static const double radiusSmall = 8;
  static const double radiusMedium = 12;
  static const double radiusLarge = 16;
  static const double radiusXLarge = 24;

  static final BorderRadius borderRadiusSmall =
      BorderRadius.circular(radiusSmall);
  static final BorderRadius borderRadiusMedium =
      BorderRadius.circular(radiusMedium);
  static final BorderRadius borderRadiusLarge =
      BorderRadius.circular(radiusLarge);
  static final BorderRadius borderRadiusXLarge =
      BorderRadius.circular(radiusXLarge);

  // ── Component sizes ──────────────────────────────────────────────────
  static const double dialpadButtonSize = 72;
  static const double callControlSize = 56;
  static const double callActionSize = 72;
  static const double avatarRadiusSmall = 16;
  static const double avatarRadiusMedium = 20;
  static const double avatarRadiusLarge = 48;
  static const double avatarRadiusXLarge = 56;

  // ── Elevation / Shadows ──────────────────────────────────────────────
  static List<BoxShadow> shadowSmall(Brightness brightness) => [
        BoxShadow(
          color: brightness == Brightness.light
              ? const Color(0x0A000000)
              : const Color(0x33000000),
          blurRadius: 8,
          offset: const Offset(0, 2),
        ),
      ];

  static List<BoxShadow> shadowMedium(Brightness brightness) => [
        BoxShadow(
          color: brightness == Brightness.light
              ? const Color(0x14000000)
              : const Color(0x4D000000),
          blurRadius: 16,
          offset: const Offset(0, 4),
        ),
      ];
}
