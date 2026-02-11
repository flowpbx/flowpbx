import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Streams network connectivity status for reactive UI updates.
///
/// Emits `true` when the device has no network connectivity (WiFi, mobile,
/// ethernet, or VPN), `false` otherwise.
final isOfflineProvider = StreamProvider<bool>((ref) {
  final connectivity = Connectivity();
  return connectivity.onConnectivityChanged.map((results) {
    return results.every((r) => r == ConnectivityResult.none);
  }).startWithCheck(connectivity);
});

extension _StartWithCheck on Stream<bool> {
  /// Checks current connectivity before yielding stream values.
  Stream<bool> startWithCheck(Connectivity connectivity) async* {
    final current = await connectivity.checkConnectivity();
    yield current.every((r) => r == ConnectivityResult.none);
    yield* this;
  }
}
