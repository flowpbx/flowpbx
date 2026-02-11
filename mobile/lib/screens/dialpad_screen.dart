import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';

class DialpadScreen extends ConsumerStatefulWidget {
  const DialpadScreen({super.key});

  @override
  ConsumerState<DialpadScreen> createState() => _DialpadScreenState();
}

class _DialpadScreenState extends ConsumerState<DialpadScreen> {
  final _numberController = TextEditingController();
  bool _isPlacingCall = false;

  @override
  void dispose() {
    _numberController.dispose();
    super.dispose();
  }

  void _appendDigit(String digit) {
    HapticFeedback.lightImpact();
    _numberController.text += digit;
    // Move cursor to end.
    _numberController.selection = TextSelection.fromPosition(
      TextPosition(offset: _numberController.text.length),
    );
  }

  void _backspace() {
    final text = _numberController.text;
    if (text.isNotEmpty) {
      HapticFeedback.lightImpact();
      _numberController.text = text.substring(0, text.length - 1);
      _numberController.selection = TextSelection.fromPosition(
        TextPosition(offset: _numberController.text.length),
      );
    }
  }

  void _clearAll() {
    _numberController.clear();
  }

  Future<void> _placeCall() async {
    final number = _numberController.text.trim();
    if (number.isEmpty) return;

    final sipService = ref.read(sipServiceProvider);
    if (!sipService.isRegistered) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Not registered — cannot place call')),
      );
      return;
    }

    setState(() => _isPlacingCall = true);
    try {
      await sipService.invite(number);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Call failed: $e')),
      );
    } finally {
      if (mounted) {
        setState(() => _isPlacingCall = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final callAsync = ref.watch(callStateProvider);
    final callState = callAsync.valueOrNull ?? ActiveCallState.idle;

    // If a call is active, the in-call screen will be shown by the router.
    // Disable the call button while already in a call.
    final hasActiveCall = callState.isActive;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Dialpad'),
      ),
      body: SafeArea(
        child: Column(
          children: [
            const SizedBox(height: 24),
            // Number display.
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 32),
              child: TextField(
                controller: _numberController,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                      letterSpacing: 2,
                    ),
                decoration: const InputDecoration(
                  border: InputBorder.none,
                  hintText: 'Enter number',
                ),
                keyboardType: TextInputType.none,
                showCursor: true,
                readOnly: true,
              ),
            ),
            const SizedBox(height: 16),
            // Dialpad grid.
            Expanded(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 40),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _buildDialpadRow(['1', '2', '3']),
                    const SizedBox(height: 12),
                    _buildDialpadRow(['4', '5', '6']),
                    const SizedBox(height: 12),
                    _buildDialpadRow(['7', '8', '9']),
                    const SizedBox(height: 12),
                    _buildDialpadRow(['*', '0', '#']),
                    const SizedBox(height: 24),
                    // Action row: call button centered, backspace on right.
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        // Spacer to balance the backspace button.
                        const SizedBox(width: 72),
                        // Call button.
                        SizedBox(
                          width: 72,
                          height: 72,
                          child: FilledButton(
                            onPressed: (hasActiveCall || _isPlacingCall)
                                ? null
                                : _placeCall,
                            style: FilledButton.styleFrom(
                              backgroundColor: Colors.green,
                              disabledBackgroundColor:
                                  Colors.green.withOpacity(0.3),
                              shape: const CircleBorder(),
                              padding: EdgeInsets.zero,
                            ),
                            child: _isPlacingCall
                                ? const SizedBox(
                                    width: 24,
                                    height: 24,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      color: Colors.white,
                                    ),
                                  )
                                : const Icon(
                                    Icons.call,
                                    size: 32,
                                    color: Colors.white,
                                  ),
                          ),
                        ),
                        // Backspace button.
                        SizedBox(
                          width: 72,
                          height: 72,
                          child: IconButton(
                            onPressed: _backspace,
                            onLongPress: _clearAll,
                            icon: Icon(
                              Icons.backspace_outlined,
                              size: 28,
                              color: colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDialpadRow(List<String> digits) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceEvenly,
      children: digits.map((d) => _DialpadButton(
        digit: d,
        subtitle: _subtitleFor(d),
        onTap: () => _appendDigit(d),
      )).toList(),
    );
  }

  static String? _subtitleFor(String digit) {
    return switch (digit) {
      '1' => null,
      '2' => 'ABC',
      '3' => 'DEF',
      '4' => 'GHI',
      '5' => 'JKL',
      '6' => 'MNO',
      '7' => 'PQRS',
      '8' => 'TUV',
      '9' => 'WXYZ',
      '0' => '+',
      _ => null,
    };
  }
}

class _DialpadButton extends StatelessWidget {
  final String digit;
  final String? subtitle;
  final VoidCallback onTap;

  const _DialpadButton({
    required this.digit,
    this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return SizedBox(
      width: 72,
      height: 72,
      child: Material(
        color: colorScheme.surfaceContainerHighest.withOpacity(0.5),
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          customBorder: const CircleBorder(),
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  digit,
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.w500,
                      ),
                ),
                if (subtitle != null)
                  Text(
                    subtitle!,
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: colorScheme.onSurfaceVariant,
                          letterSpacing: 1.5,
                          fontSize: 10,
                        ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
