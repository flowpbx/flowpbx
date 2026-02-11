import 'dart:async';
import 'dart:math';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:siprix_voip_sdk/accounts_model.dart';
import 'package:siprix_voip_sdk/calls_model.dart';
import 'package:siprix_voip_sdk/network_model.dart';
import 'package:siprix_voip_sdk/siprix_voip_sdk.dart';

import 'package:flowpbx_mobile/models/call_state.dart';
import 'package:flowpbx_mobile/services/audio_session_service.dart';
import 'package:flowpbx_mobile/services/callkit_service.dart';
import 'package:flowpbx_mobile/services/connection_service.dart';
import 'package:flowpbx_mobile/services/proximity_service.dart';
import 'package:flowpbx_mobile/services/push_service.dart';
import 'package:flowpbx_mobile/services/ringtone_service.dart';

/// Registration state for external consumers.
enum SipRegState {
  unregistered,
  registering,
  registered,
  error,
}

/// SIP registration and call service backed by siprix_voip_sdk.
class SipService {
  SipService();

  bool _initialized = false;
  int? _accountId;
  SipRegState _regState = SipRegState.unregistered;
  String _regResponse = '';
  StreamSubscription<List<ConnectivityResult>>? _connectivitySub;
  final _audioSessionService = AudioSessionService();
  final _callKitService = CallKitService();
  final _connectionService = ConnectionServiceBridge();
  final _proximityService = ProximityService();
  final _pushService = PushService();
  final _ringtoneService = RingtoneService();
  StreamSubscription<CallKitAction>? _callKitSub;
  StreamSubscription<ConnectionAction>? _connectionSub;
  StreamSubscription<PushIncomingCall>? _pushIncomingSub;

  /// Stream of VoIP push tokens for registration with the PBX API.
  Stream<String> get pushTokenStream => _pushService.tokenStream;

  /// Stream of push token invalidation events.
  Stream<void> get pushTokenInvalidatedStream =>
      _pushService.tokenInvalidatedStream;

  /// Stream of registration state changes.
  final _regStateController = StreamController<SipRegState>.broadcast();
  Stream<SipRegState> get regStateStream => _regStateController.stream;

  /// Stream of active call state changes.
  final _callStateController =
      StreamController<ActiveCallState>.broadcast();
  Stream<ActiveCallState> get callStateStream => _callStateController.stream;

  ActiveCallState _callState = ActiveCallState.idle;
  ActiveCallState get callState => _callState;

  /// Stream of audio route changes from native platform.
  Stream<AudioRoute> get audioRouteStream =>
      _audioSessionService.audioRouteStream;

  /// Query the current audio output route.
  Future<AudioRoute> getAudioRoute() => _audioSessionService.getAudioRoute();

  SipRegState get regState => _regState;
  String get regResponse => _regResponse;
  bool get isRegistered => _regState == SipRegState.registered;

  /// Initialize the Siprix SDK. Must be called once before register().
  Future<void> initialize() async {
    if (_initialized) return;

    final iniData = InitData();
    iniData.logLevelFile = LogLevel.info;
    iniData.logLevelIde = LogLevel.info;

    await SiprixVoipSdk().initialize(iniData);

    // Listen for registration state changes from the SDK.
    SiprixVoipSdk().accListener = AccStateListener(
      regStateChanged: _onRegStateChanged,
    );

    // Listen for call state changes from the SDK.
    SiprixVoipSdk().callListener = CallStateListener(
      incoming: _onCallIncoming,
      proceeding: _onCallProceeding,
      connected: _onCallConnected,
      terminated: _onCallTerminated,
      held: _onCallHeld,
      transferred: _onCallTransferred,
    );

    // Listen for network state changes from the SDK.
    SiprixVoipSdk().netListener = NetStateListener(
      networkStateChanged: _onNetworkStateChanged,
    );

    // Also monitor connectivity via connectivity_plus for faster detection.
    _connectivitySub = Connectivity().onConnectivityChanged.listen(
      _onConnectivityChanged,
    );

    // Configure iOS audio session for VoIP (AVAudioSession).
    await _audioSessionService.configure();

    // Listen for CallKit actions (iOS native call UI).
    _callKitSub = _callKitService.actionStream.listen(_onCallKitAction);

    // Listen for ConnectionService actions (Android native call UI).
    _connectionSub =
        _connectionService.actionStream.listen(_onConnectionAction);

    // Listen for push-woken incoming calls (PushKit on iOS).
    _pushIncomingSub =
        _pushService.incomingPushStream.listen(_onPushIncomingCall);

    _initialized = true;
  }

  /// Register with the PBX SIP server using credentials from auth response.
  Future<void> register({
    required String domain,
    required int port,
    required int tlsPort,
    required String username,
    required String password,
    required String transport,
  }) async {
    if (!_initialized) {
      await initialize();
    }

    // Unregister existing account if any.
    if (_accountId != null) {
      await _removeCurrentAccount();
    }

    _setRegState(SipRegState.registering);

    final account = AccountModel(
      sipServer: domain,
      sipExtension: username,
      sipPassword: password,
    );

    // Configure transport.
    final useTls = transport.toLowerCase() == 'tls';
    account.transport = useTls ? SipTransport.tls : SipTransport.tcp;

    // Use TLS port when transport is TLS, otherwise standard port.
    if (useTls && tlsPort > 0) {
      account.sipServer = '$domain:$tlsPort';
    } else if (port > 0) {
      account.sipServer = '$domain:$port';
    }

    // Enable SDES-SRTP for encrypted media (matches FlowPBX's RTP proxy).
    account.secureMedia = SecureMedia.SdesSrtp;

    // Standard SIP registration interval.
    account.expireTime = 300;

    try {
      _accountId = await SiprixVoipSdk().addAccount(account);
    } catch (e) {
      _setRegState(SipRegState.error);
      _regResponse = e.toString();
      rethrow;
    }
  }

  /// Unregister from the PBX SIP server.
  Future<void> unregister() async {
    await _removeCurrentAccount();
    _setRegState(SipRegState.unregistered);
  }

  /// Register for VoIP push notifications (iOS PushKit).
  /// Should be called after login when SIP credentials are available.
  Future<void> registerVoipPush() => _pushService.registerVoipPush();

  /// Refresh the current registration (e.g. after network change).
  Future<void> refreshRegistration() async {
    if (_accountId == null || !_initialized) return;

    _setRegState(SipRegState.registering);
    try {
      await SiprixVoipSdk().registerAccount(_accountId!);
    } catch (e) {
      _setRegState(SipRegState.error);
      _regResponse = e.toString();
    }
  }

  // -- Call management --

  /// Make an outbound call to the given destination (extension or number).
  Future<int?> invite(String destination) async {
    if (_accountId == null) throw StateError('not registered');

    await _audioSessionService.activate();

    final uuid = _generateUuid();

    _setCallState(_callState.copyWith(
      status: CallStatus.dialing,
      callUuid: uuid,
      remoteNumber: destination,
      isIncoming: false,
      isMuted: false,
      isSpeaker: false,
      isHeld: false,
      connectedAt: null,
    ));

    // Report outgoing call to CallKit (iOS) / ConnectionService (Android).
    await _callKitService.reportOutgoingCall(uuid: uuid, handle: destination);
    await _connectionService.reportOutgoingCall(uuid: uuid, handle: destination);

    try {
      final dest = CallDestination(destination, _accountId!, false);
      final callId = await SiprixVoipSdk().invite(dest);
      _setCallState(_callState.copyWith(callId: callId));
      return callId;
    } catch (e) {
      await _callKitService.reportCallEnded(uuid: uuid, reason: 2);
      await _connectionService.reportCallEnded(uuid: uuid, reason: 2);
      _setCallState(ActiveCallState.idle.copyWith(error: e.toString()));
      rethrow;
    }
  }

  /// Accept an incoming call.
  Future<void> acceptCall() async {
    final callId = _callState.callId;
    if (callId == null) return;
    _ringtoneService.stopRinging();
    await _audioSessionService.activate();
    await SiprixVoipSdk().accept(callId, false);
  }

  /// Reject an incoming call with 486 Busy.
  Future<void> rejectCall() async {
    final callId = _callState.callId;
    if (callId == null) return;
    final uuid = _callState.callUuid;
    _ringtoneService.stopRinging();
    await SiprixVoipSdk().reject(callId, 486);
    if (uuid != null) {
      await _callKitService.reportCallEnded(uuid: uuid, reason: 3);
      await _connectionService.reportCallEnded(uuid: uuid, reason: 4);
    }
    _setCallState(ActiveCallState.idle);
    await _audioSessionService.deactivate();
  }

  /// Hang up the current call.
  Future<void> hangup() async {
    final callId = _callState.callId;
    if (callId == null) return;
    final uuid = _callState.callUuid;

    _setCallState(_callState.copyWith(status: CallStatus.disconnecting));
    try {
      await SiprixVoipSdk().bye(callId);
    } catch (_) {
      // Call may already be terminated.
    }
    await _proximityService.disable();
    if (uuid != null) {
      await _callKitService.endCall(uuid: uuid);
      await _connectionService.endCall(uuid: uuid);
    }
    _setCallState(ActiveCallState.idle);
    await _audioSessionService.deactivate();
  }

  /// Toggle mute on the current call.
  Future<void> toggleMute() async {
    final callId = _callState.callId;
    if (callId == null) return;

    final newMuted = !_callState.isMuted;
    await SiprixVoipSdk().muteMic(callId, newMuted);
    _setCallState(_callState.copyWith(isMuted: newMuted));

    final uuid = _callState.callUuid;
    if (uuid != null) {
      await _callKitService.setMuted(uuid: uuid, muted: newMuted);
      await _connectionService.setMuted(uuid: uuid, muted: newMuted);
    }
  }

  /// Toggle speaker/earpiece on the current call.
  Future<void> toggleSpeaker() async {
    if (_callState.callId == null) return;

    final newSpeaker = !_callState.isSpeaker;

    // Use native audio session override on iOS for reliable speaker switching.
    await _audioSessionService.setSpeaker(newSpeaker);

    final sdk = SiprixVoipSdk();
    final count = await sdk.getPlayoutDevices() ?? 0;
    if (count >= 2) {
      // On mobile, device 0 is typically earpiece and device 1 is speaker.
      await sdk.setPlayoutDevice(newSpeaker ? 1 : 0);
    }

    _setCallState(_callState.copyWith(isSpeaker: newSpeaker));
  }

  /// Toggle hold on the current call.
  Future<void> toggleHold() async {
    final callId = _callState.callId;
    if (callId == null) return;

    await SiprixVoipSdk().hold(callId);
  }

  /// Send DTMF tones on the current call.
  Future<void> sendDtmf(String tones) async {
    final callId = _callState.callId;
    if (callId == null) return;

    await SiprixVoipSdk().sendDtmf(callId, tones, 160, 60);
  }

  /// Blind transfer the current call to the given destination.
  Future<void> transferBlind(String destination) async {
    final callId = _callState.callId;
    if (callId == null) return;

    await SiprixVoipSdk().transferBlind(callId, destination);
  }

  /// Dispose resources.
  void dispose() {
    _ringtoneService.stopRinging();
    _audioSessionService.dispose();
    _callKitService.dispose();
    _connectionService.dispose();
    _pushService.dispose();
    _callKitSub?.cancel();
    _callKitSub = null;
    _connectionSub?.cancel();
    _connectionSub = null;
    _pushIncomingSub?.cancel();
    _pushIncomingSub = null;
    _connectivitySub?.cancel();
    _connectivitySub = null;
    if (_accountId != null && _initialized) {
      // Fire-and-forget unregistration on dispose.
      SiprixVoipSdk().unRegisterAccount(_accountId!).catchError((_) {});
    }
    _regStateController.close();
    _callStateController.close();
    _initialized = false;
    _accountId = null;
  }

  // -- Private helpers --

  Future<void> _removeCurrentAccount() async {
    if (_accountId == null) return;
    try {
      await SiprixVoipSdk().deleteAccount(_accountId!);
    } catch (_) {
      // Ignore errors during cleanup.
    }
    _accountId = null;
  }

  void _setRegState(SipRegState state) {
    _regState = state;
    _regStateController.add(state);
  }

  /// Callback from siprix_voip_sdk when registration state changes.
  void _onRegStateChanged(int accId, RegState state, String response) {
    if (accId != _accountId) return;

    _regResponse = response;

    switch (state) {
      case RegState.success:
        _setRegState(SipRegState.registered);
      case RegState.failed:
        _setRegState(SipRegState.error);
      case RegState.removed:
        _setRegState(SipRegState.unregistered);
        _accountId = null;
      case RegState.inProgress:
        _setRegState(SipRegState.registering);
    }
  }

  /// Callback from siprix_voip_sdk when network state changes.
  void _onNetworkStateChanged(String name, NetState state) {
    switch (state) {
      case NetState.lost:
        _setRegState(SipRegState.error);
        _regResponse = 'network lost';
      case NetState.restored:
      case NetState.switched:
        // SDK handles re-registration internally on network restore/switch.
        // We update state to registering; the SDK callback will set registered.
        if (_accountId != null) {
          _setRegState(SipRegState.registering);
        }
    }
  }

  /// Callback from connectivity_plus for faster network change detection.
  void _onConnectivityChanged(List<ConnectivityResult> results) {
    if (_accountId == null || !_initialized) return;

    final hasConnection = results.any(
      (r) => r != ConnectivityResult.none,
    );

    if (!hasConnection) {
      _setRegState(SipRegState.error);
      _regResponse = 'no network';
    } else if (_regState != SipRegState.registered) {
      // Network restored — refresh registration.
      refreshRegistration();
    }
  }

  // -- Call state helpers --

  void _setCallState(ActiveCallState state) {
    _callState = state;
    _callStateController.add(state);
  }

  /// Callback: incoming call received.
  void _onCallIncoming(int callId, int accId, bool withVideo,
      String hdrFrom, String hdrTo) {
    // Parse caller display name from From header (e.g. "Name" <sip:ext@domain>).
    String remoteNumber = hdrFrom;
    String? displayName;
    final nameMatch = RegExp(r'"(.+?)"').firstMatch(hdrFrom);
    if (nameMatch != null) {
      displayName = nameMatch.group(1);
    }
    final extMatch = RegExp(r'sip:(\w+)@').firstMatch(hdrFrom);
    if (extMatch != null) {
      remoteNumber = extMatch.group(1)!;
    }

    // Check if this call was already reported via PushKit (push-woken call).
    // In that case, the call state has a UUID but no callId yet. Attach the
    // SIP call ID so acceptCall/rejectCall work correctly.
    if (_callState.status == CallStatus.ringing &&
        _callState.isIncoming &&
        _callState.callId == null) {
      _setCallState(_callState.copyWith(
        callId: callId,
        remoteNumber: remoteNumber,
        remoteDisplayName: displayName ?? _callState.remoteDisplayName,
      ));
      // CallKit was already notified by PushKit — don't report again.
      // Ringtone is handled by CallKit on lock screen, but start it for
      // foreground consistency.
      _ringtoneService.startRinging();
      return;
    }

    final uuid = _generateUuid();

    _setCallState(ActiveCallState(
      callId: callId,
      callUuid: uuid,
      status: CallStatus.ringing,
      remoteNumber: remoteNumber,
      remoteDisplayName: displayName,
      isIncoming: true,
    ));

    // Report incoming call to CallKit (iOS) / ConnectionService (Android).
    _callKitService.reportIncomingCall(
      uuid: uuid,
      handle: remoteNumber,
      displayName: displayName,
    );
    _connectionService.reportIncomingCall(
      uuid: uuid,
      handle: remoteNumber,
      displayName: displayName,
    );

    _ringtoneService.startRinging();
  }

  /// Callback: outbound call proceeding (100 Trying / 180 Ringing).
  void _onCallProceeding(int callId, String response) {
    if (callId != _callState.callId) return;

    // Notify CallKit that the outgoing call has started connecting so the
    // system UI shows the correct timing indicator.
    final uuid = _callState.callUuid;
    if (uuid != null && !_callState.isIncoming) {
      _callKitService.reportOutgoingCallStartedConnecting(uuid: uuid);
    }
  }

  /// Callback: call connected (200 OK — RTP flowing).
  void _onCallConnected(int callId, String hdrFrom, String hdrTo,
      bool withVideo) {
    if (callId != _callState.callId) return;
    _setCallState(_callState.copyWith(
      status: CallStatus.connected,
      connectedAt: DateTime.now(),
    ));
    _proximityService.enable();

    final uuid = _callState.callUuid;
    if (uuid != null) {
      if (!_callState.isIncoming) {
        _callKitService.reportOutgoingCallConnected(uuid: uuid);
      }
      _connectionService.reportCallConnected(uuid: uuid);
    }
  }

  /// Callback: call terminated (BYE received or sent).
  void _onCallTerminated(int callId, int statusCode) {
    if (callId != _callState.callId) return;
    final uuid = _callState.callUuid;
    _ringtoneService.stopRinging();
    _proximityService.disable();
    if (uuid != null) {
      _callKitService.reportCallEnded(uuid: uuid, reason: 1);
      _connectionService.reportCallEnded(uuid: uuid, reason: 1);
    }
    _setCallState(ActiveCallState.idle);
    _audioSessionService.deactivate();
  }

  /// Callback: hold state changed.
  void _onCallHeld(int callId, HoldState holdState) {
    if (callId != _callState.callId) return;
    final isHeld = holdState != HoldState.none;
    _setCallState(_callState.copyWith(
      status: isHeld ? CallStatus.held : CallStatus.connected,
      isHeld: isHeld,
    ));
  }

  /// Callback: call transferred.
  void _onCallTransferred(int callId, int statusCode) {
    if (callId != _callState.callId) return;
    final uuid = _callState.callUuid;
    _proximityService.disable();
    if (uuid != null) {
      _callKitService.reportCallEnded(uuid: uuid, reason: 1);
      _connectionService.reportCallEnded(uuid: uuid, reason: 1);
    }
    _setCallState(ActiveCallState.idle);
    _audioSessionService.deactivate();
  }

  /// Handle a push-woken incoming call from PushKit (iOS).
  ///
  /// When the app receives a VoIP push, the native side has already reported
  /// the call to CallKit (showing the lock screen UI). Here we set up the
  /// Dart-side call state so the SIP stack can accept the INVITE when it
  /// arrives, and trigger a SIP re-registration if needed.
  void _onPushIncomingCall(PushIncomingCall push) {
    // Set call state to ringing with info from the push payload.
    // The actual SIP INVITE will arrive after the PBX sees us re-register.
    _setCallState(ActiveCallState(
      callUuid: push.uuid,
      status: CallStatus.ringing,
      remoteNumber: push.callerId,
      remoteDisplayName: push.callerName.isNotEmpty ? push.callerName : null,
      isIncoming: true,
    ));

    // Trigger SIP re-registration so the PBX knows we're awake and can
    // route the INVITE to us. The callId will be set when the SIP INVITE
    // arrives via _onCallIncoming.
    if (_accountId != null && _regState != SipRegState.registered) {
      refreshRegistration();
    }
  }

  /// Handle actions from the native CallKit UI (iOS).
  void _onCallKitAction(CallKitAction action) {
    switch (action) {
      case CallKitAnswerAction():
        acceptCall();
      case CallKitEndAction():
        if (_callState.isActive) {
          if (_callState.status == CallStatus.ringing && _callState.isIncoming) {
            rejectCall();
          } else {
            hangup();
          }
        }
      case CallKitMuteAction(:final muted):
        if (_callState.callId != null && _callState.isMuted != muted) {
          SiprixVoipSdk().muteMic(_callState.callId!, muted);
          _setCallState(_callState.copyWith(isMuted: muted));
        }
      case CallKitHoldAction(:final held):
        if (_callState.callId != null) {
          SiprixVoipSdk().hold(_callState.callId!);
        }
      case CallKitStartCallAction(:final uuid, :final handle):
        // System-initiated outgoing call (e.g. user tapped in iOS Recents).
        // Only place the call if we're not already in one and we're registered.
        if (!_callState.isActive && _accountId != null) {
          _onSystemStartCall(uuid, handle);
        }
      case CallKitDtmfAction(:final digits):
        sendDtmf(digits);
      case CallKitResetAction():
        if (_callState.isActive) {
          _ringtoneService.stopRinging();
          _proximityService.disable();
          _setCallState(ActiveCallState.idle);
          _audioSessionService.deactivate();
        }
      case CallKitAudioActivatedAction():
        // Audio session activated by CallKit — no additional action needed.
        break;
      case CallKitAudioDeactivatedAction():
        // Audio session deactivated by CallKit — no additional action needed.
        break;
    }
  }

  /// Handle actions from the native ConnectionService UI (Android).
  void _onConnectionAction(ConnectionAction action) {
    switch (action) {
      case ConnectionAnswerAction():
        acceptCall();
      case ConnectionEndAction():
        if (_callState.isActive) {
          if (_callState.status == CallStatus.ringing && _callState.isIncoming) {
            rejectCall();
          } else {
            hangup();
          }
        }
      case ConnectionMuteAction(:final muted):
        if (_callState.callId != null && _callState.isMuted != muted) {
          SiprixVoipSdk().muteMic(_callState.callId!, muted);
          _setCallState(_callState.copyWith(isMuted: muted));
        }
      case ConnectionHoldAction(:final held):
        if (_callState.callId != null) {
          SiprixVoipSdk().hold(_callState.callId!);
        }
      case ConnectionDtmfAction(:final digits):
        sendDtmf(digits);
      case ConnectionFailedAction():
        // Connection creation failed on the system side.
        if (_callState.isActive) {
          _ringtoneService.stopRinging();
          _proximityService.disable();
          _setCallState(ActiveCallState.idle);
          _audioSessionService.deactivate();
        }
    }
  }

  /// Handle a system-initiated outgoing call (e.g. user tapped in iOS Recents).
  ///
  /// CallKit has already created the CXStartCallAction with a UUID and handle.
  /// We need to place the SIP call using that existing UUID rather than
  /// generating a new one.
  Future<void> _onSystemStartCall(String uuid, String handle) async {
    if (_accountId == null) return;

    await _audioSessionService.activate();

    _setCallState(_callState.copyWith(
      status: CallStatus.dialing,
      callUuid: uuid,
      remoteNumber: handle,
      isIncoming: false,
      isMuted: false,
      isSpeaker: false,
      isHeld: false,
      connectedAt: null,
    ));

    try {
      final dest = CallDestination(handle, _accountId!, false);
      final callId = await SiprixVoipSdk().invite(dest);
      _setCallState(_callState.copyWith(callId: callId));
    } catch (e) {
      await _callKitService.reportCallEnded(uuid: uuid, reason: 2);
      _setCallState(ActiveCallState.idle.copyWith(error: e.toString()));
    }
  }

  /// Generate a v4 UUID string.
  static String _generateUuid() {
    final rng = Random();
    final bytes = List<int>.generate(16, (_) => rng.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 1
    final hex = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
    return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-'
        '${hex.substring(12, 16)}-${hex.substring(16, 20)}-'
        '${hex.substring(20, 32)}';
  }
}
