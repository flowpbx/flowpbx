import 'dart:io';

import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/app.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialize Firebase on Android for FCM push notifications.
  if (Platform.isAndroid) {
    await Firebase.initializeApp();
  }

  runApp(
    const ProviderScope(
      child: FlowPBXApp(),
    ),
  );
}
