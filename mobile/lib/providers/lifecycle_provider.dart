import 'dart:ui';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

/// Observes app lifecycle transitions and suspends/resumes SIP registration.
///
/// When the app moves to the background with no active call, SIP is
/// unregistered to conserve battery. Incoming calls are handled via push
/// notifications (PushKit on iOS, FCM on Android) which re-wake the SIP
/// stack. When the app returns to the foreground, SIP is re-registered
/// for immediate call handling.
final lifecycleProvider = Provider<AppLifecycleObserver>((ref) {
  final sipService = ref.watch(sipServiceProvider);
  final observer = AppLifecycleObserver(
    onBackground: () => sipService.onBackground(),
    onForeground: () => sipService.onForeground(),
  );
  ref.onDispose(() => observer.dispose());
  return observer;
});

/// Bridges Flutter's [AppLifecycleListener] to SIP lifecycle callbacks.
class AppLifecycleObserver {
  AppLifecycleObserver({
    required Future<void> Function() onBackground,
    required Future<void> Function() onForeground,
  })  : _onBackground = onBackground,
        _onForeground = onForeground {
    _listener = AppLifecycleListener(
      onStateChange: _onStateChange,
    );
  }

  final Future<void> Function() _onBackground;
  final Future<void> Function() _onForeground;
  late final AppLifecycleListener _listener;

  void _onStateChange(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.detached:
        _onBackground();
      case AppLifecycleState.resumed:
        _onForeground();
      case AppLifecycleState.inactive:
      case AppLifecycleState.hidden:
        // Transitional states — no action needed.
        break;
    }
  }

  void dispose() {
    _listener.dispose();
  }
}
