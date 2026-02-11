import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
    // Keep timer running during hold so total call duration is accurate.
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
            // DTMF pad (shown when toggled during connected call).
            if (_showDtmfPad && isConnected) ...[
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 40),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _buildDtmfRow(['1', '2', '3']),
                    const SizedBox(height: 8),
                    _buildDtmfRow(['4', '5', '6']),
                    const SizedBox(height: 8),
                    _buildDtmfRow(['7', '8', '9']),
                    const SizedBox(height: 8),
                    _buildDtmfRow(['*', '0', '#']),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              TextButton(
                onPressed: () => setState(() => _showDtmfPad = false),
                child: const Text('Hide'),
              ),
              const SizedBox(height: 16),
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
                    icon: callState.isSpeaker
                        ? Icons.volume_up
                        : Icons.hearing,
                    label: callState.isSpeaker ? 'Speaker' : 'Earpiece',
                    isActive: callState.isSpeaker,
                    onPressed: _toggleSpeaker,
                  ),
                ],
              ),
              const SizedBox(height: 16),
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
              const SizedBox(height: 32),
            ],
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

  String _initials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return name.isNotEmpty ? name[0].toUpperCase() : '?';
  }
}

/// A circular call control button with icon and label.
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

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 56,
          height: 56,
          child: FilledButton.tonal(
            onPressed: onPressed,
            style: FilledButton.styleFrom(
              backgroundColor: isActive
                  ? colorScheme.primary
                  : colorScheme.surfaceContainerHighest,
              foregroundColor: isActive
                  ? colorScheme.onPrimary
                  : colorScheme.onSurface,
              shape: const CircleBorder(),
              padding: EdgeInsets.zero,
            ),
            child: Icon(icon, size: 24),
          ),
        ),
        const SizedBox(height: 8),
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

/// A circular DTMF button for the in-call keypad.
class _DtmfButton extends StatelessWidget {
  final String tone;
  final VoidCallback onTap;

  const _DtmfButton({required this.tone, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return SizedBox(
      width: 64,
      height: 64,
      child: Material(
        color: colorScheme.surfaceContainerHighest.withOpacity(0.5),
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          customBorder: const CircleBorder(),
          child: Center(
            child: Text(
              tone,
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w500,
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
