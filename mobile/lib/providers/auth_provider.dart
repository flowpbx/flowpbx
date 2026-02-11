import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/auth_state.dart';
import 'package:flowpbx_mobile/models/sip_config.dart';
import 'package:flowpbx_mobile/services/api_service.dart';
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
  @override
  Future<AuthState> build() async {
    final storage = ref.read(secureStorageProvider);
    final token = await storage.getToken();
    final serverUrl = await storage.getServerUrl();
    final extension_ = await storage.getExtension();
    final expiresAt = await storage.getExpiresAt();

    if (token != null && serverUrl != null) {
      ref.read(apiServiceProvider).setBaseUrl(serverUrl);
      return AuthState(
        token: token,
        expiresAt: expiresAt,
        serverUrl: serverUrl,
        extension_: extension_,
      );
    }
    return AuthState.empty;
  }

  /// Login: authenticate with PBX, store credentials, return SIP config.
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

    return sipConfig;
  }

  /// Logout: clear tokens and return to unauthenticated state.
  Future<void> logout() async {
    final storage = ref.read(secureStorageProvider);
    await storage.clearAll();
    state = const AsyncData(AuthState.empty);
  }
}
