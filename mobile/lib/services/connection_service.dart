import 'dart:async';
import 'dart:io';

import 'package:flutter/services.dart';

/// Dart bridge to Android ConnectionService for native call UI integration.
///
/// Provides methods to report incoming/outgoing calls to the system,
/// and receives user actions (answer, end, mute, hold, DTMF) from
/// the native Android call UI.
///
/// On iOS, all methods are no-ops (CallKit is separate).
class ConnectionServiceBridge {
  static const _channel = MethodChannel('com.flowpbx.mobile/connection');

  final _actionController = StreamController<ConnectionAction>.broadcast();

  /// Stream of user actions from the native Android call UI.
  Stream<ConnectionAction> get actionStream => _actionController.stream;

  ConnectionServiceBridge() {
    if (Platform.isAndroid) {
      _channel.setMethodCallHandler(_handleNativeCall);
    }
  }

  /// Handle calls from native ConnectionService.
  Future<dynamic> _handleNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'onConnectionAnswer':
        final args = call.arguments as Map;
        final uuid = args['uuid'] as String;
        _actionController.add(ConnectionAction.answer(uuid));
      case 'onConnectionEnd':
        final args = call.arguments as Map;
        final uuid = args['uuid'] as String;
        _actionController.add(ConnectionAction.end(uuid));
      case 'onConnectionMute':
        final args = call.arguments as Map;
        _actionController.add(ConnectionAction.mute(
          args['uuid'] as String,
          args['muted'] as bool,
        ));
      case 'onConnectionHold':
        final args = call.arguments as Map;
        _actionController.add(ConnectionAction.hold(
          args['uuid'] as String,
          args['held'] as bool,
        ));
      case 'onConnectionDTMF':
        final args = call.arguments as Map;
        _actionController.add(ConnectionAction.dtmf(
          args['uuid'] as String,
          args['digits'] as String,
        ));
      case 'onConnectionFailed':
        final args = call.arguments as Map;
        final uuid = args['uuid'] as String;
        _actionController.add(ConnectionAction.failed(uuid));
    }
  }

  /// Report an incoming call to ConnectionService (shows native UI).
  Future<bool> reportIncomingCall({
    required String uuid,
    required String handle,
    String? displayName,
  }) async {
    if (!Platform.isAndroid) return false;

    try {
      final result = await _channel.invokeMethod('reportIncomingCall', {
        'uuid': uuid,
        'handle': handle,
        'displayName': displayName,
      });
      return result == true;
    } on PlatformException {
      return false;
    }
  }

  /// Report an outgoing call to ConnectionService.
  Future<void> reportOutgoingCall({
    required String uuid,
    required String handle,
  }) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('reportOutgoingCall', {
        'uuid': uuid,
        'handle': handle,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Report that a call has connected.
  Future<void> reportCallConnected({required String uuid}) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('reportCallConnected', {
        'uuid': uuid,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Report that a call has ended.
  /// Reason codes: 1=remoteEnded, 2=error, 3=missed, 4=rejected
  Future<void> reportCallEnded({
    required String uuid,
    int reason = 1,
  }) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('reportCallEnded', {
        'uuid': uuid,
        'reason': reason,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Request ConnectionService to end a call.
  Future<void> endCall({required String uuid}) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('endCall', {'uuid': uuid});
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Update the mute state in ConnectionService.
  Future<void> setMuted({
    required String uuid,
    required bool muted,
  }) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('setMuted', {
        'uuid': uuid,
        'muted': muted,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Update the hold state in ConnectionService.
  Future<void> setHeld({
    required String uuid,
    required bool held,
  }) async {
    if (!Platform.isAndroid) return;

    try {
      await _channel.invokeMethod('setHeld', {
        'uuid': uuid,
        'held': held,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Dispose resources.
  void dispose() {
    _actionController.close();
  }
}

/// An action triggered by the user through the native Android call UI.
sealed class ConnectionAction {
  const ConnectionAction();

  const factory ConnectionAction.answer(String uuid) = ConnectionAnswerAction;
  const factory ConnectionAction.end(String uuid) = ConnectionEndAction;
  const factory ConnectionAction.mute(String uuid, bool muted) =
      ConnectionMuteAction;
  const factory ConnectionAction.hold(String uuid, bool held) =
      ConnectionHoldAction;
  const factory ConnectionAction.dtmf(String uuid, String digits) =
      ConnectionDtmfAction;
  const factory ConnectionAction.failed(String uuid) = ConnectionFailedAction;
}

class ConnectionAnswerAction extends ConnectionAction {
  final String uuid;
  const ConnectionAnswerAction(this.uuid);
}

class ConnectionEndAction extends ConnectionAction {
  final String uuid;
  const ConnectionEndAction(this.uuid);
}

class ConnectionMuteAction extends ConnectionAction {
  final String uuid;
  final bool muted;
  const ConnectionMuteAction(this.uuid, this.muted);
}

class ConnectionHoldAction extends ConnectionAction {
  final String uuid;
  final bool held;
  const ConnectionHoldAction(this.uuid, this.held);
}

class ConnectionDtmfAction extends ConnectionAction {
  final String uuid;
  final String digits;
  const ConnectionDtmfAction(this.uuid, this.digits);
}

class ConnectionFailedAction extends ConnectionAction {
  final String uuid;
  const ConnectionFailedAction(this.uuid);
}
