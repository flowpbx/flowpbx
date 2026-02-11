import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/lifecycle_provider.dart';
import 'package:flowpbx_mobile/router.dart';
import 'package:flowpbx_mobile/theme/app_theme.dart';
import 'package:flowpbx_mobile/widgets/offline_banner.dart' show OfflineBannerWrapper;

class FlowPBXApp extends ConsumerWidget {
  const FlowPBXApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);

    // Eagerly initialize lifecycle observer so it starts listening
    // for app background/foreground transitions immediately.
    ref.watch(lifecycleProvider);

    return MaterialApp.router(
      title: 'FlowPBX',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      themeMode: ThemeMode.system,
      routerConfig: router,
      builder: (context, child) {
        return OfflineBannerWrapper(child: child!);
      },
    );
  }
}
