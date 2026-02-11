import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/call_state.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

export 'package:flowpbx_mobile/models/call_state.dart'
    show ActiveCallState, CallStatus;

/// Streams the active call state for reactive UI updates.
final callStateProvider = StreamProvider<ActiveCallState>((ref) {
  final service = ref.watch(sipServiceProvider);
  return service.callStateStream.transform(
    StreamTransformer.fromHandlers(
      handleData: (data, sink) => sink.add(data),
    ),
  ).startWithValue(service.callState);
});

/// Extension to add startWithValue to any Stream.
extension _StartWith<T> on Stream<T> {
  Stream<T> startWithValue(T value) async* {
    yield value;
    yield* this;
  }
}
