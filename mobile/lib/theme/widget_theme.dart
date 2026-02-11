import 'package:flutter/material.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';

/// Component-level theme overrides applied globally.
abstract final class WidgetThemes {
  static AppBarTheme appBar(ColorScheme scheme) => AppBarTheme(
        elevation: 0,
        scrolledUnderElevation: 0,
        backgroundColor: scheme.surface,
        foregroundColor: scheme.onSurface,
        surfaceTintColor: Colors.transparent,
        centerTitle: false,
        titleTextStyle: TextStyle(
          fontSize: 20,
          fontWeight: FontWeight.w600,
          color: scheme.onSurface,
        ),
      );

  static CardThemeData card(ColorScheme scheme) => CardThemeData(
        elevation: 0,
        color: scheme.surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          side: BorderSide(color: scheme.outlineVariant.withOpacity(0.5)),
        ),
        margin: const EdgeInsets.symmetric(
          horizontal: Dimensions.space16,
          vertical: Dimensions.space8,
        ),
      );

  static InputDecorationTheme inputDecoration(ColorScheme scheme) =>
      InputDecorationTheme(
        filled: true,
        fillColor: scheme.surfaceContainer,
        border: OutlineInputBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          borderSide: BorderSide.none,
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          borderSide: BorderSide(color: scheme.primary, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          borderSide: BorderSide(color: scheme.error),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: Dimensions.borderRadiusMedium,
          borderSide: BorderSide(color: scheme.error, width: 1.5),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: Dimensions.space16,
          vertical: Dimensions.space12,
        ),
      );

  static NavigationBarThemeData navigationBar(ColorScheme scheme) =>
      NavigationBarThemeData(
        elevation: 0,
        backgroundColor: scheme.surface,
        surfaceTintColor: Colors.transparent,
        indicatorColor: scheme.primary.withOpacity(0.12),
        labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
        height: 64,
      );

  static FilledButtonThemeData filledButton(ColorScheme scheme) =>
      FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size(double.infinity, 48),
          shape: RoundedRectangleBorder(
            borderRadius: Dimensions.borderRadiusMedium,
          ),
        ),
      );

  static DividerThemeData divider(ColorScheme scheme) => DividerThemeData(
        color: scheme.outlineVariant.withOpacity(0.5),
        thickness: 1,
        space: 1,
      );

  static ListTileThemeData listTile(ColorScheme scheme) =>
      const ListTileThemeData(
        contentPadding: EdgeInsets.symmetric(
          horizontal: Dimensions.space16,
          vertical: Dimensions.space4,
        ),
      );
}
