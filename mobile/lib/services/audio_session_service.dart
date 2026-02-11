import 'dart:async';
import 'dart:io';

import 'package:flutter/services.dart';

/// Audio output route for VoIP calls.
enum AudioRoute {
  earpiece,
  speaker,
  bluetooth,
  headset,
}

/// Manages the native audio session for VoIP calls.
///
/// On iOS, configures AVAudioSession with playAndRecord category and
/// voiceChat mode for optimal two-way voice communication.
/// On Android, requests audio focus with AUDIOFOCUS_GAIN and sets
/// MODE_IN_COMMUNICATION for VoIP call audio routing.
///
/// Provides [audioRouteStream] for real-time notification when the
/// audio route changes (e.g. Bluetooth connects, headset plugged in).
class AudioSessionService {
  static const _channel = MethodChannel('com.flowpbx.mobile/audio_session');

  bool _configured = false;

  final _routeController = StreamController<AudioRoute>.broadcast();

  /// Stream of audio route changes from native platform.
  Stream<AudioRoute> get audioRouteStream => _routeController.stream;

  AudioSessionService() {
    _channel.setMethodCallHandler(_handleNativeCall);
  }

  /// Handle calls from native side (route change notifications).
  Future<dynamic> _handleNativeCall(MethodCall call) async {
    if (call.method == 'onAudioRouteChanged') {
      final route = _parseRoute(call.arguments as String?);
      _routeController.add(route);
    }
  }

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

  /// Query the current audio output route from native platform.
  Future<AudioRoute> getAudioRoute() async {
    if (!Platform.isIOS && !Platform.isAndroid) return AudioRoute.earpiece;

    try {
      final result = await _channel.invokeMethod<String>('getAudioRoute');
      return _parseRoute(result);
    } on PlatformException {
      return AudioRoute.earpiece;
    }
  }

  /// Parse a route string from native into [AudioRoute].
  AudioRoute _parseRoute(String? value) {
    return switch (value) {
      'speaker' => AudioRoute.speaker,
      'bluetooth' => AudioRoute.bluetooth,
      'headset' => AudioRoute.headset,
      _ => AudioRoute.earpiece,
    };
  }

  /// Dispose the route stream controller.
  void dispose() {
    _routeController.close();
  }
}
