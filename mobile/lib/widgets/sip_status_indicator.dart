import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

/// Small colored dot indicating SIP registration status.
class SipStatusIndicator extends ConsumerWidget {
  const SipStatusIndicator({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statusAsync = ref.watch(sipStatusProvider);

    final status = statusAsync.valueOrNull ?? SipRegState.unregistered;

    final (color, label) = switch (status) {
      SipRegState.registered => (Colors.green, 'Registered'),
      SipRegState.registering => (Colors.orange, 'Registering...'),
      SipRegState.error => (Colors.red, 'Error'),
      SipRegState.unregistered => (Colors.grey, 'Unregistered'),
    };

    return Tooltip(
      message: label,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: 4),
            Text(
              label,
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
        ),
      ),
    );
  }
}
