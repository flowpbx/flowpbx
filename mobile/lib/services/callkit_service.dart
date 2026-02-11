import 'dart:async';
import 'dart:io';

import 'package:flutter/services.dart';

/// Dart bridge to iOS CallKit for native call UI integration.
///
/// Provides methods to report incoming/outgoing calls to the system,
/// and receives user actions (answer, end, mute, hold, DTMF) from
/// the native CallKit UI.
///
/// On Android, all methods are no-ops (ConnectionService is separate).
class CallKitService {
  static const _channel = MethodChannel('com.flowpbx.mobile/callkit');

  final _actionController = StreamController<CallKitAction>.broadcast();

  /// Stream of user actions from the native CallKit UI.
  Stream<CallKitAction> get actionStream => _actionController.stream;

  CallKitService() {
    if (Platform.isIOS) {
      _channel.setMethodCallHandler(_handleNativeCall);
    }
  }

  /// Handle calls from native CallKit delegate.
  Future<dynamic> _handleNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'onCallKitAnswer':
        final uuid = call.arguments as String;
        _actionController.add(CallKitAction.answer(uuid));
      case 'onCallKitEnd':
        final uuid = call.arguments as String;
        _actionController.add(CallKitAction.end(uuid));
      case 'onCallKitMute':
        final args = call.arguments as Map;
        _actionController.add(CallKitAction.mute(
          args['uuid'] as String,
          args['muted'] as bool,
        ));
      case 'onCallKitHold':
        final args = call.arguments as Map;
        _actionController.add(CallKitAction.hold(
          args['uuid'] as String,
          args['held'] as bool,
        ));
      case 'onCallKitDTMF':
        final args = call.arguments as Map;
        _actionController.add(CallKitAction.dtmf(
          args['uuid'] as String,
          args['digits'] as String,
        ));
      case 'onCallKitStartCall':
        final args = call.arguments as Map;
        _actionController.add(CallKitAction.startCall(
          args['uuid'] as String,
          args['handle'] as String,
        ));
      case 'onCallKitReset':
        _actionController.add(const CallKitAction.reset());
      case 'onCallKitAudioActivated':
        _actionController.add(const CallKitAction.audioActivated());
      case 'onCallKitAudioDeactivated':
        _actionController.add(const CallKitAction.audioDeactivated());
    }
  }

  /// Report an incoming call to CallKit (shows native UI on lock screen).
  Future<bool> reportIncomingCall({
    required String uuid,
    required String handle,
    String? displayName,
  }) async {
    if (!Platform.isIOS) return false;

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

  /// Report an outgoing call to CallKit.
  Future<void> reportOutgoingCall({
    required String uuid,
    required String handle,
  }) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('reportOutgoingCall', {
        'uuid': uuid,
        'handle': handle,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Report that an outgoing call has started connecting (remote ringing).
  Future<void> reportOutgoingCallStartedConnecting({
    required String uuid,
  }) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('reportOutgoingCallStartedConnecting', {
        'uuid': uuid,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Report that an outgoing call has connected.
  Future<void> reportOutgoingCallConnected({required String uuid}) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('reportOutgoingCallConnected', {
        'uuid': uuid,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Report that a call has ended.
  /// Reason codes: 1=remoteEnded, 2=failed, 3=unanswered,
  ///               4=declinedElsewhere, 5=answeredElsewhere
  Future<void> reportCallEnded({
    required String uuid,
    int reason = 1,
  }) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('reportCallEnded', {
        'uuid': uuid,
        'reason': reason,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Request CallKit to end a call.
  Future<void> endCall({required String uuid}) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('endCall', {'uuid': uuid});
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Update the mute state in CallKit.
  Future<void> setMuted({
    required String uuid,
    required bool muted,
  }) async {
    if (!Platform.isIOS) return;

    try {
      await _channel.invokeMethod('setMuted', {
        'uuid': uuid,
        'muted': muted,
      });
    } on PlatformException {
      // Non-fatal.
    }
  }

  /// Update the hold state in CallKit.
  Future<void> setHeld({
    required String uuid,
    required bool held,
  }) async {
    if (!Platform.isIOS) return;

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

/// An action triggered by the user through the native CallKit UI.
sealed class CallKitAction {
  const CallKitAction();

  const factory CallKitAction.answer(String uuid) = CallKitAnswerAction;
  const factory CallKitAction.end(String uuid) = CallKitEndAction;
  const factory CallKitAction.mute(String uuid, bool muted) =
      CallKitMuteAction;
  const factory CallKitAction.hold(String uuid, bool held) =
      CallKitHoldAction;
  const factory CallKitAction.dtmf(String uuid, String digits) =
      CallKitDtmfAction;
  const factory CallKitAction.startCall(String uuid, String handle) =
      CallKitStartCallAction;
  const factory CallKitAction.reset() = CallKitResetAction;
  const factory CallKitAction.audioActivated() = CallKitAudioActivatedAction;
  const factory CallKitAction.audioDeactivated() =
      CallKitAudioDeactivatedAction;
}

class CallKitAnswerAction extends CallKitAction {
  final String uuid;
  const CallKitAnswerAction(this.uuid);
}

class CallKitEndAction extends CallKitAction {
  final String uuid;
  const CallKitEndAction(this.uuid);
}

class CallKitMuteAction extends CallKitAction {
  final String uuid;
  final bool muted;
  const CallKitMuteAction(this.uuid, this.muted);
}

class CallKitHoldAction extends CallKitAction {
  final String uuid;
  final bool held;
  const CallKitHoldAction(this.uuid, this.held);
}

class CallKitDtmfAction extends CallKitAction {
  final String uuid;
  final String digits;
  const CallKitDtmfAction(this.uuid, this.digits);
}

class CallKitStartCallAction extends CallKitAction {
  final String uuid;
  final String handle;
  const CallKitStartCallAction(this.uuid, this.handle);
}

class CallKitResetAction extends CallKitAction {
  const CallKitResetAction();
}

class CallKitAudioActivatedAction extends CallKitAction {
  const CallKitAudioActivatedAction();
}

class CallKitAudioDeactivatedAction extends CallKitAction {
  const CallKitAudioDeactivatedAction();
}
