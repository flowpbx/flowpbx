import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/services/sip_service.dart';

/// SIP registration status.
enum SipStatus {
  unregistered,
  registering,
  registered,
  error,
}

final sipServiceProvider = Provider<SipService>((ref) {
  final service = SipService();
  ref.onDispose(() => service.dispose());
  return service;
});

final sipStatusProvider = StateProvider<SipStatus>((ref) {
  return SipStatus.unregistered;
});
