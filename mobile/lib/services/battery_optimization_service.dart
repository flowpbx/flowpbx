import 'dart:io';

import 'package:flutter/services.dart';

/// Dart bridge for checking and requesting Android battery optimization
/// whitelisting (Doze mode exemption).
///
/// On Android, aggressive battery optimization can prevent the app from
/// receiving push notifications reliably or waking up the SIP stack in time.
/// This service checks whether the app is exempt from battery optimization
/// and can open the system settings to request exemption.
///
/// On iOS, all methods return safe defaults (iOS does not have user-facing
/// battery optimization whitelisting).
class BatteryOptimizationService {
  static const _channel =
      MethodChannel('com.flowpbx.mobile/battery_optimization');

  /// Check if the app is currently exempt from battery optimization.
  ///
  /// Returns `true` if exempted (whitelisted) or on iOS.
  Future<bool> isIgnoringBatteryOptimizations() async {
    if (!Platform.isAndroid) return true;

    try {
      final result =
          await _channel.invokeMethod<bool>('isIgnoringBatteryOptimizations');
      return result ?? false;
    } on PlatformException {
      return false;
    }
  }

  /// Open the system battery optimization settings for this app.
  ///
  /// On Android, this launches `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS`
  /// which shows a system dialog asking the user to whitelist the app.
  ///
  /// Returns `true` if the intent was launched successfully.
  Future<bool> requestIgnoreBatteryOptimizations() async {
    if (!Platform.isAndroid) return false;

    try {
      final result = await _channel
          .invokeMethod<bool>('requestIgnoreBatteryOptimizations');
      return result ?? false;
    } on PlatformException {
      return false;
    }
  }

  /// Open the general battery optimization settings page.
  ///
  /// Fallback if the direct request intent is not available.
  Future<bool> openBatteryOptimizationSettings() async {
    if (!Platform.isAndroid) return false;

    try {
      final result = await _channel
          .invokeMethod<bool>('openBatteryOptimizationSettings');
      return result ?? false;
    } on PlatformException {
      return false;
    }
  }
}
