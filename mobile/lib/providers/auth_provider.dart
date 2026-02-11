import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/auth_state.dart';
import 'package:flowpbx_mobile/models/sip_config.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/api_service.dart';
import 'package:flowpbx_mobile/services/call_directory_service.dart';
import 'package:flowpbx_mobile/services/secure_storage_service.dart';

final secureStorageProvider = Provider<SecureStorageService>((ref) {
  return SecureStorageService();
});

final apiServiceProvider = Provider<ApiService>((ref) {
  final storage = ref.watch(secureStorageProvider);
  return ApiService(storage: storage);
});

final authStateProvider =
    AsyncNotifierProvider<AuthNotifier, AuthState>(AuthNotifier.new);

class AuthNotifier extends AsyncNotifier<AuthState> {
  StreamSubscription<String>? _pushTokenSub;

  @override
  Future<AuthState> build() async {
    ref.onDispose(() {
      _pushTokenSub?.cancel();
      _pushTokenSub = null;
    });

    final storage = ref.read(secureStorageProvider);
    final token = await storage.getToken();
    final serverUrl = await storage.getServerUrl();
    final extension_ = await storage.getExtension();
    final expiresAt = await storage.getExpiresAt();

    if (token != null && serverUrl != null) {
      ref.read(apiServiceProvider).setBaseUrl(serverUrl);

      // Restore SIP registration from stored credentials.
      final sipConfig = await storage.getSipConfig();
      if (sipConfig['domain'] != null && sipConfig['username'] != null) {
        _registerSip(sipConfig);
      }

      // Re-register for VoIP push on app restore.
      _setupVoipPush();

      // Sync PBX directory to iOS Call Directory for caller ID lookup.
      _syncCallDirectory();

      return AuthState(
        token: token,
        expiresAt: expiresAt,
        serverUrl: serverUrl,
        extension_: extension_,
      );
    }
    return AuthState.empty;
  }

  /// Login: authenticate with PBX, store credentials, register SIP.
  Future<SipConfig> login({
    required String serverUrl,
    required String extension_,
    required String sipPassword,
  }) async {
    final api = ref.read(apiServiceProvider);
    final storage = ref.read(secureStorageProvider);

    api.setBaseUrl(serverUrl);

    final data = await api.authenticate(
      extension_: extension_,
      sipPassword: sipPassword,
    );

    final token = data['token'] as String;
    final expiresAt = DateTime.parse(data['expires_at'] as String);
    final sipData = data['sip'] as Map<String, dynamic>;
    final sipConfig = SipConfig.fromJson(sipData);

    // Store credentials securely.
    await Future.wait([
      storage.setToken(token),
      storage.setServerUrl(serverUrl),
      storage.setExtension(extension_),
      storage.setExpiresAt(expiresAt),
      storage.setSipConfig(
        domain: sipConfig.domain,
        port: sipConfig.port,
        tlsPort: sipConfig.tlsPort,
        username: sipConfig.username,
        password: sipConfig.password,
        transport: sipConfig.transport,
      ),
    ]);

    final extensionData = data['extension'] as Map<String, dynamic>?;
    final extensionName = extensionData?['name'] as String?;

    state = AsyncData(AuthState(
      token: token,
      expiresAt: expiresAt,
      serverUrl: serverUrl,
      extension_: extension_,
      extensionName: extensionName,
    ));

    // Register SIP after successful login.
    final sipService = ref.read(sipServiceProvider);
    await sipService.register(
      domain: sipConfig.domain,
      port: sipConfig.port,
      tlsPort: sipConfig.tlsPort,
      username: sipConfig.username,
      password: sipConfig.password,
      transport: sipConfig.transport,
    );

    // Register for VoIP push notifications (iOS PushKit).
    _setupVoipPush();

    // Sync PBX directory to iOS Call Directory for caller ID lookup.
    _syncCallDirectory();

    return sipConfig;
  }

  /// Logout: de-register SIP, clear tokens, return to login.
  Future<void> logout() async {
    // De-register SIP first.
    final sipService = ref.read(sipServiceProvider);
    await sipService.unregister();

    final storage = ref.read(secureStorageProvider);
    await storage.clearAll();
    state = const AsyncData(AuthState.empty);
  }

  /// Set up VoIP push registration and token forwarding to the PBX API.
  void _setupVoipPush() {
    if (!Platform.isIOS) return;

    final sipService = ref.read(sipServiceProvider);
    final api = ref.read(apiServiceProvider);

    // Listen for VoIP push token and forward it to the PBX.
    _pushTokenSub?.cancel();
    _pushTokenSub = sipService.pushTokenStream.listen((token) {
      api.registerPushToken(token: token, platform: 'ios').catchError((_) {});
    });

    // Trigger PushKit registration.
    sipService.registerVoipPush();
  }

  /// Sync PBX directory contacts to the iOS Call Directory extension for
  /// caller ID identification on incoming calls.
  void _syncCallDirectory() {
    if (!Platform.isIOS) return;

    final api = ref.read(apiServiceProvider);
    final callDir = CallDirectoryService();

    // Run asynchronously — caller ID sync is non-critical.
    () async {
      try {
        final data = await api.getDirectory();
        final contacts = data.map((e) {
          final entry = e as Map<String, dynamic>;
          return (
            number: entry['extension'] as String,
            label: entry['name'] as String,
          );
        }).toList();

        final entries = CallDirectoryService.formatEntries(
          contacts: contacts,
        );

        if (entries.isNotEmpty) {
          await callDir.updateEntries(entries);
          await callDir.reloadExtension();
        }
      } catch (_) {
        // Non-fatal: caller ID sync failure should not affect core functionality.
      }
    }();
  }

  /// Restore SIP registration from stored config map.
  void _registerSip(Map<String, String?> config) {
    final sipService = ref.read(sipServiceProvider);
    sipService.register(
      domain: config['domain']!,
      port: int.tryParse(config['port'] ?? '') ?? 5060,
      tlsPort: int.tryParse(config['tls_port'] ?? '') ?? 5061,
      username: config['username']!,
      password: config['password'] ?? '',
      transport: config['transport'] ?? 'tls',
    );
  }
}
