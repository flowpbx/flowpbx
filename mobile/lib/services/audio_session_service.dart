import 'dart:io';

import 'package:flutter/services.dart';

/// Manages the native audio session for VoIP calls.
///
/// On iOS, configures AVAudioSession with playAndRecord category and
/// voiceChat mode for optimal two-way voice communication.
/// On Android, requests audio focus with AUDIOFOCUS_GAIN and sets
/// MODE_IN_COMMUNICATION for VoIP call audio routing.
class AudioSessionService {
  static const _channel = MethodChannel('com.flowpbx.mobile/audio_session');

  bool _configured = false;

  /// Configure the audio session for VoIP. Call once during app init.
  Future<void> configure() async {
    if (!Platform.isIOS && !Platform.isAndroid) return;
    if (_configured) return;

    try {
      await _channel.invokeMethod('configure');
      _configured = true;
    } on PlatformException {
      // Non-fatal — Siprix SDK will still attempt its own configuration.
    }
  }

  /// Activate the audio session when a call begins.
  Future<void> activate() async {
    if (!Platform.isIOS && !Platform.isAndroid) return;

    if (!_configured) {
      await configure();
    }

    try {
      await _channel.invokeMethod('activate');
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Deactivate the audio session when a call ends.
  Future<void> deactivate() async {
    if (!Platform.isIOS && !Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('deactivate');
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Override audio output to speaker or earpiece.
  Future<void> setSpeaker(bool enabled) async {
    if (!Platform.isIOS && !Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('setSpeaker', {'enabled': enabled});
    } on PlatformException {
      // Non-fatal.
    }
  }
}
