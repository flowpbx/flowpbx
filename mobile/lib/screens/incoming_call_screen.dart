import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

/// Full-screen incoming call UI with caller ID, accept and reject buttons.
class IncomingCallScreen extends ConsumerWidget {
  const IncomingCallScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final callAsync = ref.watch(callStateProvider);
    final callState = callAsync.valueOrNull ?? ActiveCallState.idle;
    final colorScheme = Theme.of(context).colorScheme;

    final displayName =
        callState.remoteDisplayName ?? callState.remoteNumber;
    final subtitle =
        callState.remoteDisplayName != null ? callState.remoteNumber : null;

    return Scaffold(
      backgroundColor: colorScheme.surface,
      body: SafeArea(
        child: Column(
          children: [
            const Spacer(flex: 2),
            // Caller avatar.
            CircleAvatar(
              radius: 56,
              backgroundColor: colorScheme.primaryContainer,
              child: Text(
                _initials(displayName),
                style: TextStyle(
                  fontSize: 36,
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
              textAlign: TextAlign.center,
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
            // "Incoming Call" label.
            Text(
              'Incoming Call...',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
            ),
            const Spacer(flex: 3),
            // Accept / Reject buttons.
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 48),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  // Reject button.
                  _CallActionButton(
                    icon: Icons.call_end,
                    color: Colors.red,
                    label: 'Decline',
                    onPressed: () async {
                      HapticFeedback.mediumImpact();
                      final sip = ref.read(sipServiceProvider);
                      await sip.rejectCall();
                    },
                  ),
                  // Accept button.
                  _CallActionButton(
                    icon: Icons.call,
                    color: Colors.green,
                    label: 'Accept',
                    onPressed: () async {
                      HapticFeedback.mediumImpact();
                      final sip = ref.read(sipServiceProvider);
                      await sip.acceptCall();
                    },
                  ),
                ],
              ),
            ),
            const SizedBox(height: 48),
          ],
        ),
      ),
    );
  }

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }
}

/// Circular action button used for accept/reject on the incoming call screen.
class _CallActionButton extends StatelessWidget {
  final IconData icon;
  final Color color;
  final String label;
  final VoidCallback onPressed;

  const _CallActionButton({
    required this.icon,
    required this.color,
    required this.label,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 72,
          height: 72,
          child: FilledButton(
            onPressed: onPressed,
            style: FilledButton.styleFrom(
              backgroundColor: color,
              shape: const CircleBorder(),
              padding: EdgeInsets.zero,
            ),
            child: Icon(icon, size: 32, color: Colors.white),
          ),
        ),
        const SizedBox(height: 8),
        Text(
          label,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
        ),
      ],
    );
  }
}
