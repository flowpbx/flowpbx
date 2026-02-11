import 'dart:async';
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/audio_route_provider.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';

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
  bool _showDtmfPad = false;

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

  Future<void> _toggleMute() async {
    final sipService = ref.read(sipServiceProvider);
    await sipService.toggleMute();
  }

  Future<void> _toggleSpeaker() async {
    final sipService = ref.read(sipServiceProvider);
    await sipService.toggleSpeaker();
  }

  Future<void> _toggleHold() async {
    final sipService = ref.read(sipServiceProvider);
    await sipService.toggleHold();
  }

  Future<void> _sendDtmf(String tone) async {
    HapticFeedback.lightImpact();
    final sipService = ref.read(sipServiceProvider);
    await sipService.sendDtmf(tone);
  }

  Future<void> _showTransferDialog() async {
    final destination = await showDialog<String>(
      context: context,
      builder: (context) => _TransferDialog(),
    );
    if (destination == null || destination.isEmpty) return;

    final sipService = ref.read(sipServiceProvider);
    await sipService.transferBlind(destination);
  }

  @override
  Widget build(BuildContext context) {
    final callAsync = ref.watch(callStateProvider);
    final callState = callAsync.valueOrNull ?? ActiveCallState.idle;

    // Manage duration timer based on call state.
    if ((callState.status == CallStatus.connected ||
            callState.status == CallStatus.held ||
            callState.status == CallStatus.holding) &&
        callState.connectedAt != null) {
      _startTimer(callState.connectedAt!);
    } else if (callState.status != CallStatus.connected &&
        callState.status != CallStatus.held &&
        callState.status != CallStatus.holding) {
      _stopTimer();
    }

    final colorScheme = Theme.of(context).colorScheme;
    final displayName =
        callState.remoteDisplayName ?? callState.remoteNumber;
    final subtitle = callState.remoteDisplayName != null
        ? callState.remoteNumber
        : null;

    final isConnected = callState.status == CallStatus.connected ||
        callState.status == CallStatus.held;

    // Watch the audio route for icon updates.
    final audioRoute = ref.watch(audioRouteProvider).valueOrNull;

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
              // Caller avatar.
              GradientAvatar(
                name: displayName,
                radius: Dimensions.avatarRadiusLarge,
              ),
              const SizedBox(height: Dimensions.space24),
              // Caller name / number.
              Text(
                displayName,
                style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
              ),
              if (subtitle != null) ...[
                const SizedBox(height: Dimensions.space4),
                Text(
                  subtitle,
                  style: AppTypography.mono(
                    fontSize: 16,
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
              const SizedBox(height: Dimensions.space16),
              // Call status / duration.
              Text(
                _statusText(callState.status),
                style: AppTypography.mono(
                  fontSize: 18,
                  fontWeight: FontWeight.w500,
                  color: callState.status == CallStatus.connected
                      ? colorScheme.primary
                      : colorScheme.onSurfaceVariant,
                ),
              ),
              const Spacer(flex: 3),
              // DTMF pad.
              if (_showDtmfPad && isConnected) ...[
                Padding(
                  padding: const EdgeInsets.symmetric(
                      horizontal: Dimensions.space40),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _buildDtmfRow(['1', '2', '3']),
                      const SizedBox(height: Dimensions.space8),
                      _buildDtmfRow(['4', '5', '6']),
                      const SizedBox(height: Dimensions.space8),
                      _buildDtmfRow(['7', '8', '9']),
                      const SizedBox(height: Dimensions.space8),
                      _buildDtmfRow(['*', '0', '#']),
                    ],
                  ),
                ),
                const SizedBox(height: Dimensions.space16),
                TextButton(
                  onPressed: () => setState(() => _showDtmfPad = false),
                  child: const Text('Hide'),
                ),
                const SizedBox(height: Dimensions.space16),
              ],
              // Call controls.
              if (isConnected && !_showDtmfPad) ...[
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _CallControlButton(
                      icon: callState.isMuted ? Icons.mic_off : Icons.mic,
                      label: callState.isMuted ? 'Unmute' : 'Mute',
                      isActive: callState.isMuted,
                      onPressed: _toggleMute,
                    ),
                    _CallControlButton(
                      icon: callState.isHeld
                          ? Icons.play_arrow
                          : Icons.pause,
                      label: callState.isHeld ? 'Resume' : 'Hold',
                      isActive: callState.isHeld,
                      onPressed: _toggleHold,
                    ),
                    _CallControlButton(
                      icon: _audioRouteIcon(audioRoute, callState.isSpeaker),
                      label: _audioRouteLabel(audioRoute, callState.isSpeaker),
                      isActive: callState.isSpeaker,
                      onPressed: _toggleSpeaker,
                    ),
                  ],
                ),
                const SizedBox(height: Dimensions.space16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    _CallControlButton(
                      icon: Icons.dialpad,
                      label: 'Keypad',
                      isActive: false,
                      onPressed: () => setState(() => _showDtmfPad = true),
                    ),
                    _CallControlButton(
                      icon: Icons.phone_forwarded,
                      label: 'Transfer',
                      isActive: false,
                      onPressed: _showTransferDialog,
                    ),
                  ],
                ),
                const SizedBox(height: Dimensions.space32),
              ],
              // Hangup button.
              SizedBox(
                width: Dimensions.callActionSize,
                height: Dimensions.callActionSize,
                child: FilledButton(
                  onPressed: callState.status == CallStatus.disconnecting
                      ? null
                      : _hangup,
                  style: FilledButton.styleFrom(
                    backgroundColor: ColorTokens.callRed,
                    disabledBackgroundColor:
                        ColorTokens.callRed.withOpacity(0.3),
                    shape: const CircleBorder(),
                    padding: EdgeInsets.zero,
                    minimumSize: Size.zero,
                  ),
                  child: const Icon(
                    Icons.call_end,
                    size: 32,
                    color: Colors.white,
                  ),
                ),
              ),
              const SizedBox(height: Dimensions.space48),
            ],
          ),
        ),
      ),
    );
  }

  String _statusText(CallStatus status) {
    return switch (status) {
      CallStatus.dialing => 'Calling...',
      CallStatus.ringing => 'Ringing...',
      CallStatus.connected => _formatDuration(_duration),
      CallStatus.holding ||
      CallStatus.held => 'On Hold · ${_formatDuration(_duration)}',
      CallStatus.disconnecting => 'Ending...',
      CallStatus.idle => '',
    };
  }

  Widget _buildDtmfRow(List<String> tones) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceEvenly,
      children: tones
          .map((t) => _DtmfButton(tone: t, onTap: () => _sendDtmf(t)))
          .toList(),
    );
  }

  IconData _audioRouteIcon(AudioRoute? route, bool isSpeaker) {
    if (isSpeaker) return Icons.volume_up;
    return switch (route) {
      AudioRoute.bluetooth => Icons.bluetooth_audio,
      AudioRoute.headset => Icons.headset,
      AudioRoute.speaker => Icons.volume_up,
      _ => Icons.hearing,
    };
  }

  String _audioRouteLabel(AudioRoute? route, bool isSpeaker) {
    if (isSpeaker) return 'Speaker';
    return switch (route) {
      AudioRoute.bluetooth => 'Bluetooth',
      AudioRoute.headset => 'Headset',
      AudioRoute.speaker => 'Speaker',
      _ => 'Earpiece',
    };
  }
}

/// Frosted glass call control button.
class _CallControlButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool isActive;
  final VoidCallback onPressed;

  const _CallControlButton({
    required this.icon,
    required this.label,
    required this.isActive,
    required this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: Dimensions.callControlSize,
          height: Dimensions.callControlSize,
          child: ClipOval(
            child: BackdropFilter(
              filter: ImageFilter.blur(sigmaX: 10, sigmaY: 10),
              child: Container(
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: isActive
                      ? colorScheme.primary
                      : isDark
                          ? Colors.white.withOpacity(0.08)
                          : Colors.white.withOpacity(0.7),
                  border: Border.all(
                    color: isDark
                        ? Colors.white.withOpacity(0.1)
                        : colorScheme.outlineVariant.withOpacity(0.3),
                  ),
                ),
                child: Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: onPressed,
                    customBorder: const CircleBorder(),
                    child: Icon(
                      icon,
                      size: 24,
                      color: isActive
                          ? colorScheme.onPrimary
                          : colorScheme.onSurface,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
        const SizedBox(height: Dimensions.space8),
        Text(
          label,
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
        ),
      ],
    );
  }
}

/// DTMF button with mono font.
class _DtmfButton extends StatelessWidget {
  final String tone;
  final VoidCallback onTap;

  const _DtmfButton({required this.tone, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      width: 64,
      height: 64,
      child: Material(
        color: Colors.transparent,
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: Container(
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: isDark
                ? Colors.white.withOpacity(0.06)
                : Colors.white.withOpacity(0.8),
            border: Border.all(
              color: isDark
                  ? Colors.white.withOpacity(0.08)
                  : colorScheme.outlineVariant.withOpacity(0.3),
            ),
          ),
          child: InkWell(
            onTap: onTap,
            customBorder: const CircleBorder(),
            child: Center(
              child: Text(
                tone,
                style: AppTypography.mono(
                  fontSize: 22,
                  fontWeight: FontWeight.w500,
                  color: colorScheme.onSurface,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Dialog for entering a blind transfer destination.
class _TransferDialog extends StatefulWidget {
  @override
  State<_TransferDialog> createState() => _TransferDialogState();
}

class _TransferDialogState extends State<_TransferDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Transfer Call'),
      content: TextField(
        controller: _controller,
        autofocus: true,
        keyboardType: TextInputType.phone,
        decoration: const InputDecoration(
          labelText: 'Extension or number',
          hintText: 'e.g. 200 or +61400000000',
        ),
        onSubmitted: (value) => Navigator.of(context).pop(value.trim()),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () =>
              Navigator.of(context).pop(_controller.text.trim()),
          child: const Text('Transfer'),
        ),
      ],
    );
  }
}
