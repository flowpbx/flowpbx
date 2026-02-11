import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

/// In-call screen showing caller info, duration timer, and call controls.
class CallScreen extends ConsumerStatefulWidget {
  const CallScreen({super.key});

  @override
  ConsumerState<CallScreen> createState() => _CallScreenState();
}

class _CallScreenState extends ConsumerState<CallScreen> {
  Timer? _durationTimer;
  Duration _duration = Duration.zero;
  DateTime? _connectedAt;

  @override
  void dispose() {
    _durationTimer?.cancel();
    super.dispose();
  }

  void _startTimer(DateTime connectedAt) {
    if (_connectedAt == connectedAt) return;
    _connectedAt = connectedAt;
    _durationTimer?.cancel();
    _durationTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() {
        _duration = DateTime.now().difference(connectedAt);
      });
    });
    // Set initial duration immediately.
    _duration = DateTime.now().difference(connectedAt);
  }

  void _stopTimer() {
    _durationTimer?.cancel();
    _durationTimer = null;
    _connectedAt = null;
    _duration = Duration.zero;
  }

  String _formatDuration(Duration d) {
    final hours = d.inHours;
    final minutes = d.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = d.inSeconds.remainder(60).toString().padLeft(2, '0');
    if (hours > 0) {
      return '$hours:$minutes:$seconds';
    }
    return '$minutes:$seconds';
  }

  Future<void> _hangup() async {
    final sipService = ref.read(sipServiceProvider);
    await sipService.hangup();
  }

  @override
  Widget build(BuildContext context) {
    final callAsync = ref.watch(callStateProvider);
    final callState = callAsync.valueOrNull ?? ActiveCallState.idle;

    // Manage duration timer based on call state.
    if (callState.status == CallStatus.connected &&
        callState.connectedAt != null) {
      _startTimer(callState.connectedAt!);
    } else if (callState.status != CallStatus.connected) {
      _stopTimer();
    }

    final colorScheme = Theme.of(context).colorScheme;
    final displayName =
        callState.remoteDisplayName ?? callState.remoteNumber;
    final subtitle = callState.remoteDisplayName != null
        ? callState.remoteNumber
        : null;

    return Scaffold(
      backgroundColor: colorScheme.surface,
      body: SafeArea(
        child: Column(
          children: [
            const Spacer(flex: 2),
            // Caller avatar.
            CircleAvatar(
              radius: 48,
              backgroundColor: colorScheme.primaryContainer,
              child: Text(
                _initials(displayName),
                style: TextStyle(
                  fontSize: 32,
                  fontWeight: FontWeight.w600,
                  color: colorScheme.onPrimaryContainer,
                ),
              ),
            ),
            const SizedBox(height: 24),
            // Caller name / number.
            Text(
              displayName,
              style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
            ),
            if (subtitle != null) ...[
              const SizedBox(height: 4),
              Text(
                subtitle,
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
              ),
            ],
            const SizedBox(height: 16),
            // Call status / duration.
            Text(
              _statusText(callState.status),
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: callState.status == CallStatus.connected
                        ? colorScheme.primary
                        : colorScheme.onSurfaceVariant,
                  ),
            ),
            const Spacer(flex: 3),
            // Hangup button.
            SizedBox(
              width: 72,
              height: 72,
              child: FilledButton(
                onPressed:
                    callState.status == CallStatus.disconnecting
                        ? null
                        : _hangup,
                style: FilledButton.styleFrom(
                  backgroundColor: Colors.red,
                  disabledBackgroundColor: Colors.red.withOpacity(0.3),
                  shape: const CircleBorder(),
                  padding: EdgeInsets.zero,
                ),
                child: const Icon(
                  Icons.call_end,
                  size: 32,
                  color: Colors.white,
                ),
              ),
            ),
            const SizedBox(height: 48),
          ],
        ),
      ),
    );
  }

  String _statusText(CallStatus status) {
    return switch (status) {
      CallStatus.dialing => 'Calling...',
      CallStatus.ringing => 'Ringing...',
      CallStatus.connected => _formatDuration(_duration),
      CallStatus.holding || CallStatus.held => 'On Hold',
      CallStatus.disconnecting => 'Ending...',
      CallStatus.idle => '',
    };
  }

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }
}
