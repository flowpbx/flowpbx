import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

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
      SipRegState.registered => Colors.green,
      SipRegState.registering => Colors.orange,
      SipRegState.error => Colors.red,
      SipRegState.unregistered => Colors.grey,
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
            children: [
              // Status banner
              Container(
                margin: const EdgeInsets.all(16),
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: statusColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(
                    color: statusColor.withValues(alpha: 0.3),
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
                    const SizedBox(width: 12),
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
                              padding: const EdgeInsets.only(top: 4),
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

              // Connection details
              _SectionHeader(title: 'Connection'),
              _DetailRow(
                label: 'Server',
                value: config['domain'] ?? '—',
              ),
              _DetailRow(
                label: 'Port',
                value: config['transport']?.toLowerCase() == 'tls'
                    ? config['tls_port'] ?? '5061'
                    : config['port'] ?? '5060',
              ),
              _DetailRow(
                label: 'Transport',
                value: (config['transport'] ?? '—').toUpperCase(),
              ),
              _DetailRow(
                label: 'Username',
                value: config['username'] ?? '—',
              ),

              const Divider(),

              // Runtime state
              _SectionHeader(title: 'Runtime'),
              _DetailRow(
                label: 'Background suspended',
                value: sipService.isBackgroundSuspended ? 'Yes' : 'No',
              ),
              _DetailRow(
                label: 'SDK initialized',
                value: sipService.isRegistered ||
                        regState != SipRegState.unregistered
                    ? 'Yes'
                    : 'No',
              ),

              const Divider(),

              // Actions
              Padding(
                padding: const EdgeInsets.all(16),
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

class _SectionHeader extends StatelessWidget {
  final String title;

  const _SectionHeader({required this.title});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: Text(
        title,
        style: Theme.of(context).textTheme.labelLarge?.copyWith(
              color: Theme.of(context).colorScheme.primary,
            ),
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
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
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
              style: theme.textTheme.bodyMedium?.copyWith(
                fontFamily: 'monospace',
              ),
            ),
          ),
        ],
      ),
    );
  }
}
