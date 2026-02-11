import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/providers/missed_call_provider.dart';
import 'package:flowpbx_mobile/providers/voicemail_provider.dart';
import 'package:flowpbx_mobile/services/battery_optimization_service.dart';

/// Root shell widget that hosts the bottom NavigationBar and keeps each tab's
/// state alive via [StatefulNavigationShell].
class AppShell extends ConsumerStatefulWidget {
  final StatefulNavigationShell navigationShell;

  const AppShell({super.key, required this.navigationShell});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
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
      setState(() => _showBatteryBanner = true);
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
      setState(() => _showBatteryBanner = !isIgnoring);
    }
  }

  @override
  Widget build(BuildContext context) {
    final missedCount = ref.watch(missedCallCountProvider);
    final unreadVoicemail = ref.watch(unreadVoicemailCountProvider);
    final colorScheme = Theme.of(context).colorScheme;

    return Scaffold(
      body: Column(
        children: [
          if (_showBatteryBanner && !_bannerDismissed)
            MaterialBanner(
              padding:
                  const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              leading: Icon(
                Icons.battery_alert,
                color: colorScheme.error,
              ),
              content: const Text(
                'Battery optimization may prevent incoming calls when the '
                'app is in the background. Disable battery optimization '
                'for reliable call delivery.',
              ),
              actions: [
                TextButton(
                  onPressed: () => setState(() => _bannerDismissed = true),
                  child: const Text('Dismiss'),
                ),
                FilledButton.tonal(
                  onPressed: _requestBatteryWhitelist,
                  child: const Text('Fix Now'),
                ),
              ],
            ),
          Expanded(child: widget.navigationShell),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: widget.navigationShell.currentIndex,
        onDestinationSelected: (index) {
          widget.navigationShell.goBranch(
            index,
            initialLocation: index == widget.navigationShell.currentIndex,
          );
        },
        destinations: [
          const NavigationDestination(
            icon: Icon(Icons.dialpad_outlined),
            selectedIcon: Icon(Icons.dialpad),
            label: 'Keypad',
          ),
          NavigationDestination(
            icon: Badge(
              isLabelVisible: missedCount > 0,
              label: Text(missedCount > 99 ? '99+' : '$missedCount'),
              child: const Icon(Icons.history_outlined),
            ),
            selectedIcon: Badge(
              isLabelVisible: missedCount > 0,
              label: Text(missedCount > 99 ? '99+' : '$missedCount'),
              child: const Icon(Icons.history),
            ),
            label: 'Recents',
          ),
          const NavigationDestination(
            icon: Icon(Icons.contacts_outlined),
            selectedIcon: Icon(Icons.contacts),
            label: 'Contacts',
          ),
          NavigationDestination(
            icon: Badge(
              isLabelVisible: unreadVoicemail > 0,
              label:
                  Text(unreadVoicemail > 99 ? '99+' : '$unreadVoicemail'),
              child: const Icon(Icons.voicemail_outlined),
            ),
            selectedIcon: Badge(
              isLabelVisible: unreadVoicemail > 0,
              label:
                  Text(unreadVoicemail > 99 ? '99+' : '$unreadVoicemail'),
              child: const Icon(Icons.voicemail),
            ),
            label: 'Voicemail',
          ),
          const NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            selectedIcon: Icon(Icons.settings),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}
