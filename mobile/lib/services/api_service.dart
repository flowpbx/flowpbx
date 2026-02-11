import 'package:dio/dio.dart';
import 'package:flowpbx_mobile/services/secure_storage_service.dart';

/// HTTP client for communicating with the FlowPBX REST API.
class ApiService {
  late final Dio _dio;
  final SecureStorageService _storage;

  ApiService({required SecureStorageService storage}) : _storage = storage {
    _dio = Dio(
      BaseOptions(
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 30),
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
        },
      ),
    );

    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _storage.getToken();
          if (token != null) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          return handler.next(options);
        },
      ),
    );
  }

  /// Set the base URL for API requests.
  void setBaseUrl(String serverUrl) {
    final url = serverUrl.endsWith('/')
        ? serverUrl.substring(0, serverUrl.length - 1)
        : serverUrl;
    _dio.options.baseUrl = '$url/api/v1';
  }

  /// Authenticate with the PBX and return the raw response data.
  Future<Map<String, dynamic>> authenticate({
    required String extension_,
    required String sipPassword,
  }) async {
    final response = await _dio.post(
      '/app/auth',
      data: {
        'extension': extension_,
        'sip_password': sipPassword,
      },
    );
    return _unwrap(response);
  }

  /// Get the authenticated extension profile.
  Future<Map<String, dynamic>> getProfile() async {
    final response = await _dio.get('/app/me');
    return _unwrap(response);
  }

  /// Update follow-me settings and DND.
  Future<Map<String, dynamic>> updateProfile(
    Map<String, dynamic> data,
  ) async {
    final response = await _dio.put('/app/me', data: data);
    return _unwrap(response);
  }

  /// Register push notification token.
  Future<void> registerPushToken({
    required String token,
    required String platform,
  }) async {
    await _dio.post(
      '/app/push-token',
      data: {
        'token': token,
        'platform': platform,
      },
    );
  }

  /// Get extension directory (contact list).
  Future<List<dynamic>> getDirectory() async {
    final response = await _dio.get('/app/directory');
    final data = response.data;
    if (data is Map && data.containsKey('data')) {
      return data['data'] as List<dynamic>;
    }
    return data as List<dynamic>;
  }

  /// Get call history.
  Future<Map<String, dynamic>> getCallHistory({
    int limit = 50,
    int offset = 0,
  }) async {
    final response = await _dio.get(
      '/app/history',
      queryParameters: {'limit': limit, 'offset': offset},
    );
    return _unwrap(response);
  }

  /// Get voicemail messages.
  Future<List<dynamic>> getVoicemails() async {
    final response = await _dio.get('/app/voicemail');
    final data = response.data;
    if (data is Map && data.containsKey('data')) {
      return data['data'] as List<dynamic>;
    }
    return data as List<dynamic>;
  }

  /// Mark voicemail as read.
  Future<void> markVoicemailRead(int id) async {
    await _dio.put('/app/voicemail/$id/read');
  }

  /// Build the full URL for streaming a voicemail audio file.
  String voicemailAudioUrl(int id) {
    return '${_dio.options.baseUrl}/app/voicemail/$id/audio';
  }

  /// Get the current auth token for use in custom HTTP headers.
  Future<String?> getAuthToken() async {
    return _storage.getToken();
  }

  /// Unwrap the API envelope: { "data": ... }
  Map<String, dynamic> _unwrap(Response response) {
    final data = response.data;
    if (data is Map && data.containsKey('data')) {
      return data['data'] as Map<String, dynamic>;
    }
    return data as Map<String, dynamic>;
  }
}
