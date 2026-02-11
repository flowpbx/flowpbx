import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:siprix_voip_sdk/accounts_model.dart';
import 'package:siprix_voip_sdk/network_model.dart';
import 'package:siprix_voip_sdk/siprix_voip_sdk.dart';

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

  /// Stream of registration state changes.
  final _regStateController = StreamController<SipRegState>.broadcast();
  Stream<SipRegState> get regStateStream => _regStateController.stream;

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

  /// Dispose resources.
  void dispose() {
    _connectivitySub?.cancel();
    _connectivitySub = null;
    if (_accountId != null && _initialized) {
      // Fire-and-forget unregistration on dispose.
      SiprixVoipSdk().unRegisterAccount(_accountId!).catchError((_) {});
    }
    _regStateController.close();
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
}
