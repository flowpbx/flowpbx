import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:siprix_voip_sdk/accounts_model.dart';
import 'package:siprix_voip_sdk/calls_model.dart';
import 'package:siprix_voip_sdk/network_model.dart';
import 'package:siprix_voip_sdk/siprix_voip_sdk.dart';

import 'package:flowpbx_mobile/models/call_state.dart';
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
  final _ringtoneService = RingtoneService();

  /// Stream of registration state changes.
  final _regStateController = StreamController<SipRegState>.broadcast();
  Stream<SipRegState> get regStateStream => _regStateController.stream;

  /// Stream of active call state changes.
  final _callStateController =
      StreamController<ActiveCallState>.broadcast();
  Stream<ActiveCallState> get callStateStream => _callStateController.stream;

  ActiveCallState _callState = ActiveCallState.idle;
  ActiveCallState get callState => _callState;

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

    _setCallState(_callState.copyWith(
      status: CallStatus.dialing,
      remoteNumber: destination,
      isIncoming: false,
      isMuted: false,
      isSpeaker: false,
      isHeld: false,
      connectedAt: null,
    ));

    try {
      final dest = CallDestination(destination, _accountId!, false);
      final callId = await SiprixVoipSdk().invite(dest);
      _setCallState(_callState.copyWith(callId: callId));
      return callId;
    } catch (e) {
      _setCallState(ActiveCallState.idle.copyWith(error: e.toString()));
      rethrow;
    }
  }

  /// Accept an incoming call.
  Future<void> acceptCall() async {
    final callId = _callState.callId;
    if (callId == null) return;
    _ringtoneService.stopRinging();
    await SiprixVoipSdk().accept(callId, false);
  }

  /// Reject an incoming call with 486 Busy.
  Future<void> rejectCall() async {
    final callId = _callState.callId;
    if (callId == null) return;
    _ringtoneService.stopRinging();
    await SiprixVoipSdk().reject(callId, 486);
    _setCallState(ActiveCallState.idle);
  }

  /// Hang up the current call.
  Future<void> hangup() async {
    final callId = _callState.callId;
    if (callId == null) return;

    _setCallState(_callState.copyWith(status: CallStatus.disconnecting));
    try {
      await SiprixVoipSdk().bye(callId);
    } catch (_) {
      // Call may already be terminated.
    }
    _setCallState(ActiveCallState.idle);
  }

  /// Toggle mute on the current call.
  Future<void> toggleMute() async {
    final callId = _callState.callId;
    if (callId == null) return;

    final newMuted = !_callState.isMuted;
    await SiprixVoipSdk().muteMic(callId, newMuted);
    _setCallState(_callState.copyWith(isMuted: newMuted));
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

    _setCallState(ActiveCallState(
      callId: callId,
      status: CallStatus.ringing,
      remoteNumber: remoteNumber,
      remoteDisplayName: displayName,
      isIncoming: true,
    ));

    _ringtoneService.startRinging();
  }

  /// Callback: outbound call proceeding (100 Trying / 180 Ringing).
  void _onCallProceeding(int callId, String response) {
    if (callId != _callState.callId) return;
    // Stay in dialing state — the remote side is ringing.
  }

  /// Callback: call connected (200 OK — RTP flowing).
  void _onCallConnected(int callId, String hdrFrom, String hdrTo,
      bool withVideo) {
    if (callId != _callState.callId) return;
    _setCallState(_callState.copyWith(
      status: CallStatus.connected,
      connectedAt: DateTime.now(),
    ));
  }

  /// Callback: call terminated (BYE received or sent).
  void _onCallTerminated(int callId, int statusCode) {
    if (callId != _callState.callId) return;
    _ringtoneService.stopRinging();
    _setCallState(ActiveCallState.idle);
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
    _setCallState(ActiveCallState.idle);
  }
}
