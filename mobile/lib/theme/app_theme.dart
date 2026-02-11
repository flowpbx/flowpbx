import 'package:flutter/material.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/theme/widget_theme.dart';

/// Assembles the complete [ThemeData] for light and dark modes.
abstract final class AppTheme {
  static ThemeData light() {
    final scheme = ColorTokens.lightScheme();
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      colorScheme: scheme,
      scaffoldBackgroundColor: ColorTokens.lightBackground,
      textTheme: AppTypography.textTheme(Brightness.light),
      appBarTheme: WidgetThemes.appBar(scheme),
      cardTheme: WidgetThemes.card(scheme),
      inputDecorationTheme: WidgetThemes.inputDecoration(scheme),
      navigationBarTheme: WidgetThemes.navigationBar(scheme),
      filledButtonTheme: WidgetThemes.filledButton(scheme),
      dividerTheme: WidgetThemes.divider(scheme),
      listTileTheme: WidgetThemes.listTile(scheme),
    );
  }

  static ThemeData dark() {
    final scheme = ColorTokens.darkScheme();
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      colorScheme: scheme,
      scaffoldBackgroundColor: ColorTokens.darkBackground,
      textTheme: AppTypography.textTheme(Brightness.dark),
      appBarTheme: WidgetThemes.appBar(scheme),
      cardTheme: WidgetThemes.card(scheme),
      inputDecorationTheme: WidgetThemes.inputDecoration(scheme),
      navigationBarTheme: WidgetThemes.navigationBar(scheme),
      filledButtonTheme: WidgetThemes.filledButton(scheme),
      dividerTheme: WidgetThemes.divider(scheme),
      listTileTheme: WidgetThemes.listTile(scheme),
    );
  }
}
