import 'dart:io';

import 'package:flutter/services.dart';

/// Controls the proximity sensor to turn the screen off when the phone
/// is held against the user's ear during a call.
///
/// On iOS, enables UIDevice.current.isProximityMonitoringEnabled.
/// On Android, acquires a PROXIMITY_SCREEN_OFF_WAKE_LOCK.
class ProximityService {
  static const _channel = MethodChannel('com.flowpbx.mobile/proximity');

  bool _enabled = false;

  /// Enable proximity sensor monitoring (screen off when near ear).
  Future<void> enable() async {
    if (!Platform.isIOS && !Platform.isAndroid) return;
    if (_enabled) return;

    try {
      await _channel.invokeMethod('enable');
      _enabled = true;
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Disable proximity sensor monitoring.
  Future<void> disable() async {
    if (!Platform.isIOS && !Platform.isAndroid) return;
    if (!_enabled) return;

    try {
      await _channel.invokeMethod('disable');
      _enabled = false;
    } on PlatformException {
      // Non-fatal.
    }
  }
}
