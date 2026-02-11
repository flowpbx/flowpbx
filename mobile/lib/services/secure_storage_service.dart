import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Secure token and credential storage using platform keychain/keystore.
class SecureStorageService {
  static const _keyToken = 'auth_token';
  static const _keyServerUrl = 'server_url';
  static const _keyExtension = 'extension';
  static const _keyExpiresAt = 'expires_at';
  static const _keySipDomain = 'sip_domain';
  static const _keySipPort = 'sip_port';
  static const _keySipTlsPort = 'sip_tls_port';
  static const _keySipUsername = 'sip_username';
  static const _keySipPassword = 'sip_password';
  static const _keySipTransport = 'sip_transport';

  final FlutterSecureStorage _storage;

  SecureStorageService()
      : _storage = const FlutterSecureStorage(
          aOptions: AndroidOptions(encryptedSharedPreferences: true),
          iOptions: IOSOptions(
            accessibility: KeychainAccessibility.first_unlock_this_device,
          ),
        );

  // Auth token

  Future<String?> getToken() async {
    return _storage.read(key: _keyToken);
  }

  Future<void> setToken(String token) async {
    await _storage.write(key: _keyToken, value: token);
  }

  // Server URL

  Future<String?> getServerUrl() async {
    return _storage.read(key: _keyServerUrl);
  }

  Future<void> setServerUrl(String url) async {
    await _storage.write(key: _keyServerUrl, value: url);
  }

  // Extension number

  Future<String?> getExtension() async {
    return _storage.read(key: _keyExtension);
  }

  Future<void> setExtension(String ext) async {
    await _storage.write(key: _keyExtension, value: ext);
  }

  // Token expiry

  Future<DateTime?> getExpiresAt() async {
    final value = await _storage.read(key: _keyExpiresAt);
    if (value == null) return null;
    return DateTime.tryParse(value);
  }

  Future<void> setExpiresAt(DateTime expiresAt) async {
    await _storage.write(
      key: _keyExpiresAt,
      value: expiresAt.toIso8601String(),
    );
  }

  // SIP config

  Future<void> setSipConfig({
    required String domain,
    required int port,
    required int tlsPort,
    required String username,
    required String password,
    required String transport,
  }) async {
    await Future.wait([
      _storage.write(key: _keySipDomain, value: domain),
      _storage.write(key: _keySipPort, value: port.toString()),
      _storage.write(key: _keySipTlsPort, value: tlsPort.toString()),
      _storage.write(key: _keySipUsername, value: username),
      _storage.write(key: _keySipPassword, value: password),
      _storage.write(key: _keySipTransport, value: transport),
    ]);
  }

  Future<Map<String, String?>> getSipConfig() async {
    final results = await Future.wait([
      _storage.read(key: _keySipDomain),
      _storage.read(key: _keySipPort),
      _storage.read(key: _keySipTlsPort),
      _storage.read(key: _keySipUsername),
      _storage.read(key: _keySipPassword),
      _storage.read(key: _keySipTransport),
    ]);
    return {
      'domain': results[0],
      'port': results[1],
      'tls_port': results[2],
      'username': results[3],
      'password': results[4],
      'transport': results[5],
    };
  }

  // Clear all stored data (logout)

  Future<void> clearAll() async {
    await _storage.deleteAll();
  }
}
