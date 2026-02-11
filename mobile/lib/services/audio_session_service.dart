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

/// Audio interruption type from the native platform.
///
/// [began] — another audio source has interrupted (e.g. cellular call, Siri).
/// [ended] — the interruption has ended and audio can be resumed.
/// [focusLost] — Android-specific transient audio focus loss.
enum AudioInterruption {
  began,
  ended,
  focusLost,
}

/// Manages the native audio session for VoIP calls.
///
/// On iOS, configures AVAudioSession with playAndRecord category and
/// voiceChat mode for optimal two-way voice communication. Observes
/// interruption notifications (cellular call, Siri) and media services
/// reset events for graceful recovery.
///
/// On Android, requests audio focus with AUDIOFOCUS_GAIN and sets
/// MODE_IN_COMMUNICATION for VoIP call audio routing. Listens for
/// audio focus changes (transient loss, ducking) to hold/resume calls.
///
/// Provides [audioRouteStream] for real-time notification when the
/// audio route changes (e.g. Bluetooth connects, headset plugged in),
/// and [interruptionStream] for audio session interruption events.
class AudioSessionService {
  static const _channel = MethodChannel('com.flowpbx.mobile/audio_session');

  bool _configured = false;

  final _routeController = StreamController<AudioRoute>.broadcast();
  final _interruptionController = StreamController<AudioInterruption>.broadcast();

  /// Stream of audio route changes from native platform.
  Stream<AudioRoute> get audioRouteStream => _routeController.stream;

  /// Stream of audio interruption events from native platform.
  ///
  /// Consumers should hold the active call on [AudioInterruption.began]
  /// or [AudioInterruption.focusLost], and resume on [AudioInterruption.ended].
  Stream<AudioInterruption> get interruptionStream =>
      _interruptionController.stream;

  AudioSessionService() {
    _channel.setMethodCallHandler(_handleNativeCall);
  }

  /// Handle calls from native side (route change and interruption notifications).
  Future<dynamic> _handleNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'onAudioRouteChanged':
        final route = _parseRoute(call.arguments as String?);
        _routeController.add(route);
      case 'onAudioInterruption':
        final type = call.arguments as String?;
        final interruption = switch (type) {
          'began' => AudioInterruption.began,
          'ended' => AudioInterruption.ended,
          'focusLost' => AudioInterruption.focusLost,
          _ => null,
        };
        if (interruption != null) {
          _interruptionController.add(interruption);
        }
      case 'onMediaServicesReset':
        // Media server crashed and restarted (iOS). Reconfigure the session.
        _configured = false;
        await configure();
        // Notify consumers that audio was interrupted so they can recover.
        _interruptionController.add(AudioInterruption.began);
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

  /// Dispose stream controllers.
  void dispose() {
    _routeController.close();
    _interruptionController.close();
  }
}
