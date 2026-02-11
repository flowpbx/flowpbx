import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';
import 'package:flowpbx_mobile/providers/profile_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/app_error.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/error_banner.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';
import 'package:flowpbx_mobile/widgets/section_card.dart';
import 'package:flowpbx_mobile/widgets/skeleton_loader.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final profileAsync = ref.watch(profileProvider);
    final authState = ref.watch(authStateProvider);
    final sipStatus = ref.watch(sipStatusProvider);

    final serverUrl = authState.valueOrNull?.serverUrl ?? '';
    final extensionNumber = authState.valueOrNull?.extension_ ?? '';
    final regState = sipStatus.valueOrNull ?? SipRegState.unregistered;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
      ),
      body: profileAsync.when(
        loading: () => const SettingsSkeleton(),
        error: (error, _) => ErrorBanner(
          error: error,
          fallbackMessage: 'Failed to load profile',
          onRetry: () => ref.read(profileProvider.notifier).refresh(),
        ),
        data: (profile) {
          if (profile == null) {
            return const Center(child: Text('Not authenticated'));
          }

          return RefreshIndicator(
            onRefresh: () => ref.read(profileProvider.notifier).refresh(),
            child: ListView(
              padding: const EdgeInsets.symmetric(vertical: Dimensions.space8),
              children: [
                // Profile hero card.
                Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: Dimensions.space16,
                    vertical: Dimensions.space8,
                  ),
                  child: Container(
                    padding: const EdgeInsets.all(Dimensions.space20),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.surface,
                      borderRadius: Dimensions.borderRadiusLarge,
                      border: Border.all(
                        color: Theme.of(context)
                            .colorScheme
                            .outlineVariant
                            .withOpacity(0.5),
                      ),
                    ),
                    child: Row(
                      children: [
                        GradientAvatar(
                          name: profile.name,
                          radius: 32,
                        ),
                        const SizedBox(width: Dimensions.space16),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                profile.name,
                                style: Theme.of(context)
                                    .textTheme
                                    .titleMedium
                                    ?.copyWith(fontWeight: FontWeight.w600),
                              ),
                              const SizedBox(height: Dimensions.space4),
                              Text(
                                'Ext. ${profile.extension_}',
                                style: AppTypography.mono(
                                  fontSize: 14,
                                  color: Theme.of(context)
                                      .colorScheme
                                      .onSurfaceVariant,
                                ),
                              ),
                              if (profile.email.isNotEmpty) ...[
                                const SizedBox(height: Dimensions.space2),
                                Text(
                                  profile.email,
                                  style: Theme.of(context)
                                      .textTheme
                                      .bodySmall
                                      ?.copyWith(
                                        color: Theme.of(context)
                                            .colorScheme
                                            .onSurfaceVariant,
                                      ),
                                ),
                              ],
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),

                const SizedBox(height: Dimensions.space8),

                // Call Settings section.
                SectionCard(
                  title: 'Call Settings',
                  children: [
                    _DndToggle(dnd: profile.dnd),
                    Divider(height: 1, color: Theme.of(context).colorScheme.outlineVariant.withOpacity(0.3)),
                    _FollowMeToggle(enabled: profile.followMeEnabled),
                    if (profile.followMeEnabled &&
                        profile.followMeNumbers.isNotEmpty) ...[
                      Divider(height: 1, color: Theme.of(context).colorScheme.outlineVariant.withOpacity(0.3)),
                      ListTile(
                        leading: const SizedBox(width: 24),
                        title: const Text('Forward numbers'),
                        subtitle: Text(profile.followMeNumbers.join(', ')),
                      ),
                    ],
                  ],
                ),

                // Connection section.
                SectionCard(
                  title: 'Connection',
                  children: [
                    ListTile(
                      leading: const Icon(Icons.cloud_outlined),
                      title: const Text('Server'),
                      subtitle: Text(serverUrl),
                    ),
                    Divider(height: 1, color: Theme.of(context).colorScheme.outlineVariant.withOpacity(0.3)),
                    ListTile(
                      leading: Icon(
                        Icons.circle,
                        size: 12,
                        color: switch (regState) {
                          SipRegState.registered => ColorTokens.registeredGreen,
                          SipRegState.registering =>
                            ColorTokens.registeringOrange,
                          SipRegState.error => ColorTokens.errorRed,
                          SipRegState.unregistered => ColorTokens.offlineGrey,
                        },
                      ),
                      title: const Text('SIP Registration'),
                      subtitle: Text(switch (regState) {
                        SipRegState.registered =>
                          'Registered as $extensionNumber',
                        SipRegState.registering => 'Registering...',
                        SipRegState.error => 'Registration failed',
                        SipRegState.unregistered => 'Not registered',
                      }),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () => context.push('/settings/sip'),
                    ),
                  ],
                ),

                // Account section.
                SectionCard(
                  title: 'Account',
                  children: [
                    ListTile(
                      leading: const Icon(Icons.swap_horiz),
                      title: const Text('Change server'),
                      subtitle:
                          const Text('Sign out and connect to a different PBX'),
                      onTap: () => _confirmLogout(context, ref),
                    ),
                  ],
                ),

                // About section.
                SectionCard(
                  title: 'About',
                  children: const [
                    ListTile(
                      leading: Icon(Icons.info_outline),
                      title: Text('FlowPBX'),
                      subtitle: Text('Version 1.0.0'),
                    ),
                  ],
                ),

                const SizedBox(height: Dimensions.space16),
              ],
            ),
          );
        },
      ),
    );
  }

  Future<void> _confirmLogout(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign out?'),
        content: const Text(
          'You will be signed out and can connect to a different server.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('Sign out'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      await ref.read(authStateProvider.notifier).logout();
      if (context.mounted) {
        context.go('/login');
      }
    }
  }
}

class _DndToggle extends ConsumerStatefulWidget {
  final bool dnd;

  const _DndToggle({required this.dnd});

  @override
  ConsumerState<_DndToggle> createState() => _DndToggleState();
}

class _DndToggleState extends ConsumerState<_DndToggle> {
  bool _updating = false;

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      secondary: Icon(
        Icons.do_not_disturb_on_outlined,
        color:
            widget.dnd ? Theme.of(context).colorScheme.error : null,
      ),
      title: const Text('Do Not Disturb'),
      subtitle:
          Text(widget.dnd ? 'All calls are rejected' : 'Accepting calls'),
      value: widget.dnd,
      onChanged: _updating
          ? null
          : (value) async {
              setState(() => _updating = true);
              try {
                await ref.read(profileProvider.notifier).toggleDnd(value);
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                        content:
                            Text('Failed to update DND. ${formatError(e)}')),
                  );
                }
              } finally {
                if (mounted) setState(() => _updating = false);
              }
            },
    );
  }
}

class _FollowMeToggle extends ConsumerStatefulWidget {
  final bool enabled;

  const _FollowMeToggle({required this.enabled});

  @override
  ConsumerState<_FollowMeToggle> createState() => _FollowMeToggleState();
}

class _FollowMeToggleState extends ConsumerState<_FollowMeToggle> {
  bool _updating = false;

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      secondary: const Icon(Icons.phone_forwarded_outlined),
      title: const Text('Follow Me'),
      subtitle: Text(
        widget.enabled
            ? 'Calls forwarded to external numbers'
            : 'No call forwarding',
      ),
      value: widget.enabled,
      onChanged: _updating
          ? null
          : (value) async {
              setState(() => _updating = true);
              try {
                await ref
                    .read(profileProvider.notifier)
                    .toggleFollowMe(value);
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                        content: Text(
                            'Failed to update follow-me. ${formatError(e)}')),
                  );
                }
              } finally {
                if (mounted) setState(() => _updating = false);
              }
            },
    );
  }
}
