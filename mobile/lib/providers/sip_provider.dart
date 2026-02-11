import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/services/sip_service.dart';

/// Re-export for convenience in UI code.
export 'package:flowpbx_mobile/services/sip_service.dart' show SipRegState;

final sipServiceProvider = Provider<SipService>((ref) {
  final service = SipService();
  ref.onDispose(() => service.dispose());
  return service;
});

/// Streams the SIP registration state for reactive UI updates.
final sipStatusProvider = StreamProvider<SipRegState>((ref) {
  final service = ref.watch(sipServiceProvider);
  // Emit the current state immediately, then follow the stream.
  return service.regStateStream.transform(
    StreamTransformer.fromHandlers(
      handleData: (data, sink) => sink.add(data),
    ),
  ).startWithValue(service.regState);
});

/// Extension to add startWithValue to any Stream.
extension _StartWith<T> on Stream<T> {
  Stream<T> startWithValue(T value) async* {
    yield value;
    yield* this;
  }
}
