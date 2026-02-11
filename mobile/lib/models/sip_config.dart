/// SIP configuration returned from the PBX auth endpoint.
class SipConfig {
  final String domain;
  final int port;
  final int tlsPort;
  final String username;
  final String password;
  final String transport;

  const SipConfig({
    required this.domain,
    required this.port,
    required this.tlsPort,
    required this.username,
    required this.password,
    required this.transport,
  });

  factory SipConfig.fromJson(Map<String, dynamic> json) {
    return SipConfig(
      domain: json['domain'] as String,
      port: json['port'] as int,
      tlsPort: json['tls_port'] as int,
      username: json['username'] as String,
      password: json['password'] as String,
      transport: json['transport'] as String? ?? 'tls',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'domain': domain,
      'port': port,
      'tls_port': tlsPort,
      'username': username,
      'password': password,
      'transport': transport,
    };
  }
}
