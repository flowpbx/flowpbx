import 'dart:async';
import 'dart:io';

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/services.dart';

/// Top-level handler for FCM background messages (Android).
///
/// Must be a top-level function (not a class method) for firebase_messaging.
/// This runs in an isolate when the app is killed or backgrounded.
@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // Background data messages are handled when the app wakes up and
  // re-registers SIP. The push payload triggers the heads-up notification
  // via ConnectionService on the native side. No Dart processing needed
  // here — the SIP stack will handle the call once registration completes.
}

/// Dart bridge for push notification handling on both platforms.
///
/// iOS: Uses PushKit (VoIP push) via native method channel.
/// Android: Uses Firebase Cloud Messaging (FCM) via firebase_messaging plugin.
///
/// Provides:
/// - Push token delivery to Dart for PBX registration
/// - Incoming push-woken call notifications for SIP wake-up
class PushService {
  static const _channel = MethodChannel('com.flowpbx.mobile/push');

  final _tokenController = StreamController<String>.broadcast();
  final _tokenInvalidatedController = StreamController<void>.broadcast();
  final _incomingPushController =
      StreamController<PushIncomingCall>.broadcast();

  StreamSubscription<String>? _fcmTokenRefreshSub;
  StreamSubscription<RemoteMessage>? _fcmMessageSub;
  StreamSubscription<RemoteMessage>? _fcmOpenedAppSub;

  /// Stream of push tokens (VoIP token on iOS, FCM token on Android).
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

  /// Handle method calls from native PushKit delegate (iOS only).
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

  /// Register for FCM push notifications (Android only).
  /// Should be called after successful login/authentication.
  Future<void> registerFcmPush() async {
    if (!Platform.isAndroid) return;

    // Set up background message handler.
    FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);

    // Request notification permission (Android 13+).
    await FirebaseMessaging.instance.requestPermission(
      alert: true,
      badge: false,
      sound: true,
      criticalAlert: true,
    );

    // Set high-priority delivery for data messages.
    await FirebaseMessaging.instance.setForegroundNotificationPresentationOptions(
      alert: false,
      badge: false,
      sound: false,
    );

    // Get current FCM token and emit it.
    final token = await FirebaseMessaging.instance.getToken();
    if (token != null) {
      _tokenController.add(token);
    }

    // Listen for token refresh events (e.g. app reinstall, token rotation).
    _fcmTokenRefreshSub?.cancel();
    _fcmTokenRefreshSub =
        FirebaseMessaging.instance.onTokenRefresh.listen((newToken) {
      _tokenController.add(newToken);
    });

    // Listen for foreground FCM data messages containing incoming call info.
    _fcmMessageSub?.cancel();
    _fcmMessageSub =
        FirebaseMessaging.onMessage.listen(_handleFcmMessage);

    // Check if the app was launched from a terminated state by an FCM message.
    final initialMessage = await FirebaseMessaging.instance.getInitialMessage();
    if (initialMessage != null) {
      _handleFcmMessage(initialMessage);
    }

    // Handle messages that opened the app from background (not terminated).
    _fcmOpenedAppSub?.cancel();
    _fcmOpenedAppSub =
        FirebaseMessaging.onMessageOpenedApp.listen(_handleFcmMessage);
  }

  /// Handle an incoming FCM data message (foreground).
  void _handleFcmMessage(RemoteMessage message) {
    final data = message.data;

    // Only process call push notifications from the PBX push gateway.
    if (data['type'] != 'incoming_call') return;

    final callerId = data['caller_id'] as String? ?? '';
    final callerName = data['caller_name'] as String? ?? '';
    final callId = data['call_id'] as String? ?? '';

    _incomingPushController.add(PushIncomingCall(
      uuid: callId.isNotEmpty ? callId : DateTime.now().millisecondsSinceEpoch.toString(),
      callerId: callerId,
      callerName: callerName,
      callId: callId,
    ));
  }

  /// Dispose resources.
  void dispose() {
    _fcmTokenRefreshSub?.cancel();
    _fcmTokenRefreshSub = null;
    _fcmMessageSub?.cancel();
    _fcmMessageSub = null;
    _fcmOpenedAppSub?.cancel();
    _fcmOpenedAppSub = null;
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
