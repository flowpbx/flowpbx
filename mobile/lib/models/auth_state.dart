/// Authentication state for the app.
class AuthState {
  final String? token;
  final DateTime? expiresAt;
  final String? serverUrl;
  final String? extension_;
  final String? extensionName;

  const AuthState({
    this.token,
    this.expiresAt,
    this.serverUrl,
    this.extension_,
    this.extensionName,
  });

  bool get isAuthenticated => token != null && !isExpired;

  bool get isExpired {
    if (expiresAt == null) return true;
    return DateTime.now().isAfter(expiresAt!);
  }

  AuthState copyWith({
    String? token,
    DateTime? expiresAt,
    String? serverUrl,
    String? extension_,
    String? extensionName,
  }) {
    return AuthState(
      token: token ?? this.token,
      expiresAt: expiresAt ?? this.expiresAt,
      serverUrl: serverUrl ?? this.serverUrl,
      extension_: extension_ ?? this.extension_,
      extensionName: extensionName ?? this.extensionName,
    );
  }

  static const empty = AuthState();
}
