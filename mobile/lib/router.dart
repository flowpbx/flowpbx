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

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authStateProvider);
  final callState = ref.watch(callStateProvider);
  final hasActiveCall =
      callState.valueOrNull?.isActive ?? false;

  return GoRouter(
    initialLocation: '/',
    redirect: (context, state) {
      final isAuthenticated = authState.valueOrNull?.isAuthenticated ?? false;
      final isLoginRoute = state.matchedLocation == '/login';
      final isCallRoute = state.matchedLocation == '/call';

      if (!isAuthenticated && !isLoginRoute) {
        return '/login';
      }
      if (isAuthenticated && isLoginRoute) {
        return '/';
      }

      // Redirect to call screen when a call is active.
      if (isAuthenticated && hasActiveCall && !isCallRoute) {
        return '/call';
      }

      // Redirect away from call screen when no call is active.
      if (isCallRoute && !hasActiveCall) {
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
        path: '/call',
        builder: (context, state) => const CallScreen(),
      ),
    ],
  );
});
