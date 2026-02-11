import 'dart:io';

import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/app.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialize Firebase on Android for FCM push notifications.
  if (Platform.isAndroid) {
    await Firebase.initializeApp();
  }

  // Allow flutter_animate to restart animations on hot reload during
  // development for faster iteration.
  if (kDebugMode) {
    Animate.restartOnHotReload = true;
  }

  runApp(
    const ProviderScope(
      child: FlowPBXApp(),
    ),
  );
}
