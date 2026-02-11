import 'package:flutter/material.dart';

/// Semantic color tokens for the FlowPBX design system.
abstract final class ColorTokens {
  // ── Light theme ──────────────────────────────────────────────────────
  static const lightBackground = Color(0xFFFAFBFD);
  static const lightSurface = Color(0xFFFFFFFF);
  static const lightPrimary = Color(0xFF2563EB);
  static const lightOnPrimary = Color(0xFFFFFFFF);
  static const lightPrimaryContainer = Color(0xFFDBEAFE);
  static const lightOnPrimaryContainer = Color(0xFF1E40AF);
  static const lightSuccess = Color(0xFF16A34A);
  static const lightError = Color(0xFFDC2626);
  static const lightErrorContainer = Color(0xFFFEE2E2);
  static const lightOnErrorContainer = Color(0xFF991B1B);
  static const lightTextPrimary = Color(0xFF1A1F2E);
  static const lightTextSecondary = Color(0xFF64748B);
  static const lightDivider = Color(0xFFE2E8F0);
  static const lightSurfaceContainer = Color(0xFFF1F5F9);
  static const lightSurfaceContainerHigh = Color(0xFFE8ECF1);

  // ── Dark theme ───────────────────────────────────────────────────────
  static const darkBackground = Color(0xFF0F1117);
  static const darkSurface = Color(0xFF1A1D27);
  static const darkPrimary = Color(0xFF60A5FA);
  static const darkOnPrimary = Color(0xFF0F1117);
  static const darkPrimaryContainer = Color(0xFF1E3A5F);
  static const darkOnPrimaryContainer = Color(0xFFBFDBFE);
  static const darkSuccess = Color(0xFF4ADE80);
  static const darkError = Color(0xFFF87171);
  static const darkErrorContainer = Color(0xFF3B1111);
  static const darkOnErrorContainer = Color(0xFFFCA5A5);
  static const darkTextPrimary = Color(0xFFE2E8F0);
  static const darkTextSecondary = Color(0xFF94A3B8);
  static const darkDivider = Color(0xFF2D3348);
  static const darkSurfaceContainer = Color(0xFF232736);
  static const darkSurfaceContainerHigh = Color(0xFF2D3141);

  // ── Shared functional colors ─────────────────────────────────────────
  static const callGreen = Color(0xFF16A34A);
  static const callRed = Color(0xFFDC2626);
  static const registeredGreen = Color(0xFF16A34A);
  static const registeringOrange = Color(0xFFF59E0B);
  static const errorRed = Color(0xFFDC2626);
  static const offlineGrey = Color(0xFF94A3B8);

  /// Build a full light [ColorScheme] from design tokens.
  static ColorScheme lightScheme() => const ColorScheme.light(
        primary: lightPrimary,
        onPrimary: lightOnPrimary,
        primaryContainer: lightPrimaryContainer,
        onPrimaryContainer: lightOnPrimaryContainer,
        secondary: lightPrimary,
        onSecondary: lightOnPrimary,
        error: lightError,
        onError: lightOnPrimary,
        errorContainer: lightErrorContainer,
        onErrorContainer: lightOnErrorContainer,
        surface: lightSurface,
        onSurface: lightTextPrimary,
        onSurfaceVariant: lightTextSecondary,
        surfaceContainerLowest: lightBackground,
        surfaceContainerLow: lightBackground,
        surfaceContainer: lightSurfaceContainer,
        surfaceContainerHigh: lightSurfaceContainerHigh,
        surfaceContainerHighest: lightSurfaceContainerHigh,
        outline: lightDivider,
        outlineVariant: lightDivider,
      );

  /// Build a full dark [ColorScheme] from design tokens.
  static ColorScheme darkScheme() => const ColorScheme.dark(
        primary: darkPrimary,
        onPrimary: darkOnPrimary,
        primaryContainer: darkPrimaryContainer,
        onPrimaryContainer: darkOnPrimaryContainer,
        secondary: darkPrimary,
        onSecondary: darkOnPrimary,
        error: darkError,
        onError: darkOnPrimary,
        errorContainer: darkErrorContainer,
        onErrorContainer: darkOnErrorContainer,
        surface: darkSurface,
        onSurface: darkTextPrimary,
        onSurfaceVariant: darkTextSecondary,
        surfaceContainerLowest: darkBackground,
        surfaceContainerLow: darkBackground,
        surfaceContainer: darkSurfaceContainer,
        surfaceContainerHigh: darkSurfaceContainerHigh,
        surfaceContainerHighest: darkSurfaceContainerHigh,
        outline: darkDivider,
        outlineVariant: darkDivider,
      );
}
