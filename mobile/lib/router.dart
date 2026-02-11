import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/screens/login_screen.dart';
import 'package:flowpbx_mobile/screens/home_screen.dart';
import 'package:flowpbx_mobile/screens/contacts_screen.dart';
import 'package:flowpbx_mobile/screens/dialpad_screen.dart';
import 'package:flowpbx_mobile/screens/call_screen.dart';
import 'package:flowpbx_mobile/screens/call_history_screen.dart';
import 'package:flowpbx_mobile/screens/incoming_call_screen.dart';
import 'package:flowpbx_mobile/screens/settings_screen.dart';
import 'package:flowpbx_mobile/screens/voicemail_screen.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authStateProvider);
  final callState = ref.watch(callStateProvider);
  final call = callState.valueOrNull;
  final hasActiveCall = call?.isActive ?? false;
  final isIncomingRinging = call != null &&
      call.isIncoming &&
      call.status == CallStatus.ringing;

  return GoRouter(
    initialLocation: '/',
    redirect: (context, state) {
      final isAuthenticated = authState.valueOrNull?.isAuthenticated ?? false;
      final isLoginRoute = state.matchedLocation == '/login';
      final isCallRoute = state.matchedLocation == '/call';
      final isIncomingRoute = state.matchedLocation == '/incoming';

      if (!isAuthenticated && !isLoginRoute) {
        return '/login';
      }
      if (isAuthenticated && isLoginRoute) {
        return '/';
      }

      // Redirect to incoming call screen when ringing with an inbound call.
      if (isAuthenticated && isIncomingRinging && !isIncomingRoute) {
        return '/incoming';
      }

      // Redirect to in-call screen for active calls that are not incoming ringing.
      if (isAuthenticated &&
          hasActiveCall &&
          !isIncomingRinging &&
          !isCallRoute) {
        return '/call';
      }

      // Redirect away from call/incoming screens when no call is active.
      if ((isCallRoute || isIncomingRoute) && !hasActiveCall) {
        return '/';
      }

      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        builder: (context, state) => const LoginScreen(),
      ),
      GoRoute(
        path: '/',
        builder: (context, state) => const HomeScreen(),
      ),
      GoRoute(
        path: '/dialpad',
        builder: (context, state) {
          final number = state.uri.queryParameters['number'];
          return DialpadScreen(initialNumber: number);
        },
      ),
      GoRoute(
        path: '/contacts',
        builder: (context, state) => const ContactsScreen(),
      ),
      GoRoute(
        path: '/history',
        builder: (context, state) => const CallHistoryScreen(),
      ),
      GoRoute(
        path: '/voicemail',
        builder: (context, state) => const VoicemailScreen(),
      ),
      GoRoute(
        path: '/settings',
        builder: (context, state) => const SettingsScreen(),
      ),
      GoRoute(
        path: '/call',
        builder: (context, state) => const CallScreen(),
      ),
      GoRoute(
        path: '/incoming',
        builder: (context, state) => const IncomingCallScreen(),
      ),
    ],
  );
});
