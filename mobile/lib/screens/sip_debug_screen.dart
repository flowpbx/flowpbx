import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/section_card.dart';

class SipDebugScreen extends ConsumerWidget {
  const SipDebugScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sipStatus = ref.watch(sipStatusProvider);
    final sipService = ref.read(sipServiceProvider);
    final storage = ref.read(secureStorageProvider);
    final theme = Theme.of(context);
    final regState = sipStatus.valueOrNull ?? SipRegState.unregistered;

    final statusColor = switch (regState) {
      SipRegState.registered => ColorTokens.registeredGreen,
      SipRegState.registering => ColorTokens.registeringOrange,
      SipRegState.error => ColorTokens.errorRed,
      SipRegState.unregistered => ColorTokens.offlineGrey,
    };

    final statusLabel = switch (regState) {
      SipRegState.registered => 'Registered',
      SipRegState.registering => 'Registering...',
      SipRegState.error => 'Failed',
      SipRegState.unregistered => 'Unregistered',
    };

    return Scaffold(
      appBar: AppBar(
        title: const Text('SIP Status'),
      ),
      body: FutureBuilder<Map<String, String?>>(
        future: storage.getSipConfig(),
        builder: (context, snapshot) {
          final config = snapshot.data ?? {};

          return ListView(
            padding: const EdgeInsets.symmetric(vertical: Dimensions.space8),
            children: [
              // Status banner.
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: Dimensions.space16,
                  vertical: Dimensions.space8,
                ),
                child: Container(
                  padding: const EdgeInsets.all(Dimensions.space16),
                  decoration: BoxDecoration(
                    color: statusColor.withOpacity(0.1),
                    borderRadius: Dimensions.borderRadiusMedium,
                    border: Border.all(
                      color: statusColor.withOpacity(0.3),
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        regState == SipRegState.registered
                            ? Icons.check_circle
                            : regState == SipRegState.registering
                                ? Icons.sync
                                : regState == SipRegState.error
                                    ? Icons.error
                                    : Icons.circle_outlined,
                        color: statusColor,
                        size: 32,
                      ),
                      const SizedBox(width: Dimensions.space12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              statusLabel,
                              style: theme.textTheme.titleMedium?.copyWith(
                                color: statusColor,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            if (sipService.regResponse.isNotEmpty)
                              Padding(
                                padding:
                                    const EdgeInsets.only(top: Dimensions.space4),
                                child: Text(
                                  sipService.regResponse,
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    color: theme.colorScheme.onSurfaceVariant,
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),

              // Connection details.
              SectionCard(
                title: 'Connection',
                children: [
                  _DetailRow(
                    label: 'Server',
                    value: config['domain'] ?? '\u2014',
                  ),
                  Divider(height: 1, color: theme.colorScheme.outlineVariant.withOpacity(0.3)),
                  _DetailRow(
                    label: 'Port',
                    value: config['transport']?.toLowerCase() == 'tls'
                        ? config['tls_port'] ?? '5061'
                        : config['port'] ?? '5060',
                  ),
                  Divider(height: 1, color: theme.colorScheme.outlineVariant.withOpacity(0.3)),
                  _DetailRow(
                    label: 'Transport',
                    value: (config['transport'] ?? '\u2014').toUpperCase(),
                  ),
                  Divider(height: 1, color: theme.colorScheme.outlineVariant.withOpacity(0.3)),
                  _DetailRow(
                    label: 'Username',
                    value: config['username'] ?? '\u2014',
                  ),
                ],
              ),

              // Runtime state.
              SectionCard(
                title: 'Runtime',
                children: [
                  _DetailRow(
                    label: 'Background suspended',
                    value: sipService.isBackgroundSuspended ? 'Yes' : 'No',
                  ),
                  Divider(height: 1, color: theme.colorScheme.outlineVariant.withOpacity(0.3)),
                  _DetailRow(
                    label: 'SDK initialized',
                    value: sipService.isRegistered ||
                            regState != SipRegState.unregistered
                        ? 'Yes'
                        : 'No',
                  ),
                ],
              ),

              // Actions.
              Padding(
                padding: const EdgeInsets.all(Dimensions.space16),
                child: FilledButton.icon(
                  onPressed: regState == SipRegState.registering
                      ? null
                      : () => sipService.refreshRegistration(),
                  icon: const Icon(Icons.refresh),
                  label: const Text('Refresh Registration'),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: Dimensions.space16,
        vertical: Dimensions.space12,
      ),
      child: Row(
        children: [
          SizedBox(
            width: 160,
            child: Text(
              label,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: AppTypography.mono(
                fontSize: 14,
                color: theme.colorScheme.onSurface,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
