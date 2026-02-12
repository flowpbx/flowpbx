import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/screens/app_shell.dart';
import 'package:flowpbx_mobile/screens/login_screen.dart';
import 'package:flowpbx_mobile/screens/dialpad_screen.dart';
import 'package:flowpbx_mobile/screens/call_history_screen.dart';
import 'package:flowpbx_mobile/screens/contacts_screen.dart';
import 'package:flowpbx_mobile/screens/voicemail_screen.dart';
import 'package:flowpbx_mobile/screens/settings_screen.dart';
import 'package:flowpbx_mobile/screens/sip_debug_screen.dart';
import 'package:flowpbx_mobile/screens/call_screen.dart';
import 'package:flowpbx_mobile/screens/incoming_call_screen.dart';

final _rootNavigatorKey = GlobalKey<NavigatorState>();

/// A [ChangeNotifier] that fires whenever auth or call state changes,
/// triggering GoRouter to re-evaluate its redirect.
class _RouterRefreshNotifier extends ChangeNotifier {
  _RouterRefreshNotifier(Ref ref) {
    ref.listen(authStateProvider, (_, __) => notifyListeners());
    ref.listen(callStateProvider, (_, __) => notifyListeners());
  }
}

final routerProvider = Provider<GoRouter>((ref) {
  final refreshNotifier = _RouterRefreshNotifier(ref);

  return GoRouter(
    navigatorKey: _rootNavigatorKey,
    initialLocation: '/dialpad',
    refreshListenable: refreshNotifier,
    redirect: (context, state) {
      final authState = ref.read(authStateProvider);
      final callState = ref.read(callStateProvider);
      final call = callState.valueOrNull;
      final hasActiveCall = call?.isActive ?? false;
      final isIncomingRinging = call != null &&
          call.isIncoming &&
          call.status == CallStatus.ringing;

      final isAuthenticated =
          authState.valueOrNull?.isAuthenticated ?? false;
      final isLoginRoute = state.matchedLocation == '/login';
      final isCallRoute = state.matchedLocation == '/call';
      final isIncomingRoute = state.matchedLocation == '/incoming';

      if (!isAuthenticated && !isLoginRoute) {
        return '/login';
      }
      if (isAuthenticated && isLoginRoute) {
        return '/dialpad';
      }

      // Redirect to incoming call screen when ringing with an inbound call.
      if (isAuthenticated && isIncomingRinging && !isIncomingRoute) {
        return '/incoming';
      }

      // Redirect to in-call screen for active calls that are not incoming
      // ringing.
      if (isAuthenticated &&
          hasActiveCall &&
          !isIncomingRinging &&
          !isCallRoute) {
        return '/call';
      }

      // Redirect away from call/incoming screens when no call is active.
      if ((isCallRoute || isIncomingRoute) && !hasActiveCall) {
        return '/dialpad';
      }

      return null;
    },
    routes: [
      // Login — full-screen, no bottom nav.
      GoRoute(
        path: '/login',
        parentNavigatorKey: _rootNavigatorKey,
        pageBuilder: (context, state) => CustomTransitionPage(
          key: state.pageKey,
          child: const LoginScreen(),
          transitionsBuilder: (context, animation, _, child) =>
              FadeTransition(opacity: animation, child: child),
        ),
      ),

      // Call — full-screen overlay, slides up.
      GoRoute(
        path: '/call',
        parentNavigatorKey: _rootNavigatorKey,
        pageBuilder: (context, state) => CustomTransitionPage(
          key: state.pageKey,
          child: const CallScreen(),
          transitionsBuilder: (context, animation, _, child) =>
              SlideTransition(
            position: Tween<Offset>(
              begin: const Offset(0, 1),
              end: Offset.zero,
            ).animate(CurvedAnimation(
              parent: animation,
              curve: Curves.easeOutCubic,
            )),
            child: child,
          ),
        ),
      ),

      // Incoming call — full-screen overlay, slides up.
      GoRoute(
        path: '/incoming',
        parentNavigatorKey: _rootNavigatorKey,
        pageBuilder: (context, state) => CustomTransitionPage(
          key: state.pageKey,
          child: const IncomingCallScreen(),
          transitionsBuilder: (context, animation, _, child) =>
              SlideTransition(
            position: Tween<Offset>(
              begin: const Offset(0, 1),
              end: Offset.zero,
            ).animate(CurvedAnimation(
              parent: animation,
              curve: Curves.easeOutCubic,
            )),
            child: child,
          ),
        ),
      ),

      // Main tab shell with 5 branches.
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) =>
            AppShell(navigationShell: navigationShell),
        branches: [
          // Tab 0: Keypad
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/dialpad',
                builder: (context, state) {
                  final number = state.uri.queryParameters['number'];
                  return DialpadScreen(initialNumber: number);
                },
              ),
            ],
          ),

          // Tab 1: Recents
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/history',
                builder: (context, state) => const CallHistoryScreen(),
              ),
            ],
          ),

          // Tab 2: Contacts
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/contacts',
                builder: (context, state) => const ContactsScreen(),
              ),
            ],
          ),

          // Tab 3: Voicemail
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/voicemail',
                builder: (context, state) => const VoicemailScreen(),
              ),
            ],
          ),

          // Tab 4: Settings
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/settings',
                builder: (context, state) => const SettingsScreen(),
                routes: [
                  GoRoute(
                    path: 'sip',
                    builder: (context, state) => const SipDebugScreen(),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    ],
  );
});
