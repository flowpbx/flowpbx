import 'dart:async';
import 'dart:io';

import 'package:flutter/services.dart';

/// Dart bridge to native PushKit (iOS) for VoIP push notification handling.
///
/// Provides:
/// - VoIP push registration via PushKit
/// - Token delivery to Dart for PBX registration
/// - Incoming push-woken call notifications for SIP wake-up
///
/// On Android, all methods are no-ops (FCM is used instead).
class PushService {
  static const _channel = MethodChannel('com.flowpbx.mobile/push');

  final _tokenController = StreamController<String>.broadcast();
  final _tokenInvalidatedController = StreamController<void>.broadcast();
  final _incomingPushController =
      StreamController<PushIncomingCall>.broadcast();

  /// Stream of VoIP push tokens from PushKit.
  Stream<String> get tokenStream => _tokenController.stream;

  /// Stream emitted when the push token is invalidated.
  Stream<void> get tokenInvalidatedStream => _tokenInvalidatedController.stream;

  /// Stream of push-woken incoming calls (call info from push payload).
  Stream<PushIncomingCall> get incomingPushStream =>
      _incomingPushController.stream;

  PushService() {
    if (Platform.isIOS) {
      _channel.setMethodCallHandler(_handleNativeCall);
    }
  }

  /// Handle method calls from native PushKit delegate.
  Future<dynamic> _handleNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'onVoipToken':
        final token = call.arguments as String;
        _tokenController.add(token);
      case 'onVoipTokenInvalidated':
        _tokenInvalidatedController.add(null);
      case 'onPushIncomingCall':
        final args = call.arguments as Map;
        _incomingPushController.add(PushIncomingCall(
          uuid: args['uuid'] as String,
          callerId: args['caller_id'] as String,
          callerName: args['caller_name'] as String,
          callId: args['call_id'] as String,
        ));
    }
  }

  /// Register for VoIP push notifications via PushKit (iOS only).
  /// Should be called after successful login/authentication.
  Future<void> registerVoipPush() async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('registerVoipPush');
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Dispose resources.
  void dispose() {
    _tokenController.close();
    _tokenInvalidatedController.close();
    _incomingPushController.close();
  }
}

/// Data from a push-woken incoming call notification.
class PushIncomingCall {
  final String uuid;
  final String callerId;
  final String callerName;
  final String callId;

  const PushIncomingCall({
    required this.uuid,
    required this.callerId,
    required this.callerName,
    required this.callId,
  });
}
