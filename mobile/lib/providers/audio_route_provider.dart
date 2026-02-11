import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/audio_session_service.dart';

export 'package:flowpbx_mobile/services/audio_session_service.dart'
    show AudioRoute;

/// Streams the current audio output route for reactive UI updates.
final audioRouteProvider = StreamProvider<AudioRoute>((ref) {
  final service = ref.watch(sipServiceProvider);
  return service.audioRouteStream;
});
