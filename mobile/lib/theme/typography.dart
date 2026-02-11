import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

/// App-wide typography built on Plus Jakarta Sans (primary) and
/// JetBrains Mono (accent / mono).
abstract final class AppTypography {
  /// Full [TextTheme] using Plus Jakarta Sans for all roles.
  static TextTheme textTheme(Brightness brightness) {
    final base = GoogleFonts.plusJakartaSansTextTheme(
      brightness == Brightness.light
          ? ThemeData.light().textTheme
          : ThemeData.dark().textTheme,
    );
    return base.copyWith(
      // Slightly heavier headings for visual punch.
      displayLarge: base.displayLarge?.copyWith(fontWeight: FontWeight.w700),
      displayMedium: base.displayMedium?.copyWith(fontWeight: FontWeight.w700),
      displaySmall: base.displaySmall?.copyWith(fontWeight: FontWeight.w700),
      headlineLarge: base.headlineLarge?.copyWith(fontWeight: FontWeight.w700),
      headlineMedium: base.headlineMedium?.copyWith(fontWeight: FontWeight.w600),
      headlineSmall: base.headlineSmall?.copyWith(fontWeight: FontWeight.w600),
      titleLarge: base.titleLarge?.copyWith(fontWeight: FontWeight.w600),
      titleMedium: base.titleMedium?.copyWith(fontWeight: FontWeight.w600),
    );
  }

  /// Returns a mono [TextStyle] via JetBrains Mono for dialpad digits,
  /// call timers, and extension numbers.
  static TextStyle mono({
    double fontSize = 14,
    FontWeight fontWeight = FontWeight.w500,
    Color? color,
    double? letterSpacing,
    double? height,
  }) {
    return GoogleFonts.jetBrainsMono(
      fontSize: fontSize,
      fontWeight: fontWeight,
      color: color,
      letterSpacing: letterSpacing,
      height: height,
    );
  }
}
