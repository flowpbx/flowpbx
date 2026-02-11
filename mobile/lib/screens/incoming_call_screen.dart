import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';

/// Full-screen incoming call UI with caller ID, accept and reject buttons,
/// and pulsing ripple animation behind the avatar.
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
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              colorScheme.primary.withOpacity(0.08),
              colorScheme.surface,
            ],
          ),
        ),
        child: SafeArea(
          child: Column(
            children: [
              const Spacer(flex: 2),
              // Pulsing ripple circles behind avatar.
              SizedBox(
                width: 200,
                height: 200,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    // 3 concentric ripples with staggered loop.
                    for (var i = 0; i < 3; i++)
                      Container(
                        width: 200,
                        height: 200,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(
                            color:
                                colorScheme.primary.withOpacity(0.15),
                            width: 1.5,
                          ),
                        ),
                      )
                          .animate(
                              onPlay: (c) => c.repeat())
                          .scaleXY(
                            begin: 0.5,
                            end: 1.0,
                            duration: 2000.ms,
                            delay: (i * 600).ms,
                            curve: Curves.easeOut,
                          )
                          .fadeOut(
                            begin: 0.8,
                            duration: 2000.ms,
                            delay: (i * 600).ms,
                          ),
                    // Actual avatar on top.
                    GradientAvatar(
                      name: displayName,
                      radius: Dimensions.avatarRadiusXLarge,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: Dimensions.space24),
              // Caller name / number.
              Text(
                displayName,
                style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                textAlign: TextAlign.center,
              ),
              if (subtitle != null) ...[
                const SizedBox(height: Dimensions.space4),
                Text(
                  subtitle,
                  style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                        color: colorScheme.onSurfaceVariant,
                      ),
                ),
              ],
              const SizedBox(height: Dimensions.space16),
              Text(
                'Incoming Call...',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
              ),
              const Spacer(flex: 3),
              // Accept / Reject buttons.
              Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: Dimensions.space48),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _CallActionButton(
                      icon: Icons.call_end,
                      color: ColorTokens.callRed,
                      label: 'Decline',
                      onPressed: () async {
                        HapticFeedback.mediumImpact();
                        final sip = ref.read(sipServiceProvider);
                        await sip.rejectCall();
                      },
                    ),
                    _CallActionButton(
                      icon: Icons.call,
                      color: ColorTokens.callGreen,
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
              const SizedBox(height: Dimensions.space48),
            ],
          ),
        ),
      ),
    );
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
          width: Dimensions.callActionSize,
          height: Dimensions.callActionSize,
          child: FilledButton(
            onPressed: onPressed,
            style: FilledButton.styleFrom(
              backgroundColor: color,
              shape: const CircleBorder(),
              padding: EdgeInsets.zero,
              minimumSize: Size.zero,
            ),
            child: Icon(icon, size: 32, color: Colors.white),
          ),
        ),
        const SizedBox(height: Dimensions.space8),
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
