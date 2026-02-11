import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';

/// Small colored dot indicating SIP registration status.
class SipStatusIndicator extends ConsumerWidget {
  const SipStatusIndicator({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statusAsync = ref.watch(sipStatusProvider);

    final status = statusAsync.valueOrNull ?? SipRegState.unregistered;

    final (color, label) = switch (status) {
      SipRegState.registered => (ColorTokens.registeredGreen, 'Registered'),
      SipRegState.registering =>
        (ColorTokens.registeringOrange, 'Registering...'),
      SipRegState.error => (ColorTokens.errorRed, 'Error'),
      SipRegState.unregistered => (ColorTokens.offlineGrey, 'Unregistered'),
    };

    Widget dot = Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(
        color: color,
        shape: BoxShape.circle,
      ),
    );

    // Pulse while registering.
    if (status == SipRegState.registering) {
      dot = dot
          .animate(onPlay: (c) => c.repeat(reverse: true))
          .scaleXY(begin: 1.0, end: 0.6, duration: 800.ms);
    }

    return Tooltip(
      message: label,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            dot,
            const SizedBox(width: 4),
            Text(
              label,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: color,
                    fontWeight: FontWeight.w500,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
