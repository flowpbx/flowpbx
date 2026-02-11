import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/battery_optimization_service.dart';
import 'package:flowpbx_mobile/widgets/sip_status_indicator.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  final _batteryService = BatteryOptimizationService();
  bool _showBatteryBanner = false;
  bool _bannerDismissed = false;

  @override
  void initState() {
    super.initState();
    _checkBatteryOptimization();
  }

  Future<void> _checkBatteryOptimization() async {
    if (!Platform.isAndroid) return;

    final isIgnoring = await _batteryService.isIgnoringBatteryOptimizations();
    if (mounted && !isIgnoring) {
      setState(() {
        _showBatteryBanner = true;
      });
    }
  }

  Future<void> _requestBatteryWhitelist() async {
    final success =
        await _batteryService.requestIgnoreBatteryOptimizations();
    if (!success) {
      await _batteryService.openBatteryOptimizationSettings();
    }

    // Re-check after returning from settings.
    await Future.delayed(const Duration(seconds: 1));
    if (!mounted) return;

    final isIgnoring = await _batteryService.isIgnoringBatteryOptimizations();
    if (mounted) {
      setState(() {
        _showBatteryBanner = !isIgnoring;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authStateProvider);
    final extensionName = authState.valueOrNull?.extensionName;
    final extension_ = authState.valueOrNull?.extension_;

    return Scaffold(
      appBar: AppBar(
        title: const Text('FlowPBX'),
        actions: [
          const SipStatusIndicator(),
          IconButton(
            icon: const Icon(Icons.history),
            tooltip: 'Call History',
            onPressed: () => context.push('/history'),
          ),
          IconButton(
            icon: const Icon(Icons.contacts_outlined),
            tooltip: 'Contacts',
            onPressed: () => context.push('/contacts'),
          ),
          IconButton(
            icon: const Icon(Icons.logout),
            tooltip: 'Sign out',
            onPressed: () async {
              await ref.read(authStateProvider.notifier).logout();
            },
          ),
        ],
      ),
      body: Column(
        children: [
          if (_showBatteryBanner && !_bannerDismissed)
            MaterialBanner(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              leading: Icon(
                Icons.battery_alert,
                color: Theme.of(context).colorScheme.error,
              ),
              content: const Text(
                'Battery optimization may prevent incoming calls when the '
                'app is in the background. Disable battery optimization '
                'for reliable call delivery.',
              ),
              actions: [
                TextButton(
                  onPressed: () {
                    setState(() {
                      _bannerDismissed = true;
                    });
                  },
                  child: const Text('Dismiss'),
                ),
                FilledButton.tonal(
                  onPressed: _requestBatteryWhitelist,
                  child: const Text('Fix Now'),
                ),
              ],
            ),
          Expanded(
            child: Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(
                    Icons.phone_in_talk,
                    size: 80,
                    color:
                        Theme.of(context).colorScheme.primary.withOpacity(0.3),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    extensionName ?? 'Extension $extension_',
                    style: Theme.of(context).textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Ext. ${extension_ ?? ''}',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color:
                              Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.large(
        heroTag: 'dialpad',
        onPressed: () => context.go('/dialpad'),
        backgroundColor: Colors.green,
        child: const Icon(Icons.dialpad, size: 32, color: Colors.white),
      ),
      floatingActionButtonLocation: FloatingActionButtonLocation.centerFloat,
    );
  }
}
