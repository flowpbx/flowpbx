import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/directory_entry.dart';
import 'package:flowpbx_mobile/providers/call_provider.dart';
import 'package:flowpbx_mobile/providers/directory_provider.dart';
import 'package:flowpbx_mobile/providers/sip_provider.dart';
import 'package:flowpbx_mobile/services/app_error.dart';
import 'package:flowpbx_mobile/theme/color_tokens.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';
import 'package:flowpbx_mobile/theme/typography.dart';
import 'package:flowpbx_mobile/widgets/gradient_avatar.dart';
import 'package:flowpbx_mobile/widgets/sip_status_indicator.dart';

class DialpadScreen extends ConsumerStatefulWidget {
  final String? initialNumber;

  const DialpadScreen({super.key, this.initialNumber});

  @override
  ConsumerState<DialpadScreen> createState() => _DialpadScreenState();
}

class _DialpadScreenState extends ConsumerState<DialpadScreen> {
  final _numberController = TextEditingController();
  bool _isPlacingCall = false;
  List<DirectoryEntry> _matchingContacts = [];

  /// T9 keypad mapping: letter -> digit.
  static const _t9Map = <String, String>{
    'a': '2', 'b': '2', 'c': '2',
    'd': '3', 'e': '3', 'f': '3',
    'g': '4', 'h': '4', 'i': '4',
    'j': '5', 'k': '5', 'l': '5',
    'm': '6', 'n': '6', 'o': '6',
    'p': '7', 'q': '7', 'r': '7', 's': '7',
    't': '8', 'u': '8', 'v': '8',
    'w': '9', 'x': '9', 'y': '9', 'z': '9',
  };

  @override
  void initState() {
    super.initState();
    if (widget.initialNumber != null && widget.initialNumber!.isNotEmpty) {
      _numberController.text = widget.initialNumber!;
    }
    _numberController.addListener(_updateContactMatches);
  }

  @override
  void dispose() {
    _numberController.removeListener(_updateContactMatches);
    _numberController.dispose();
    super.dispose();
  }

  /// Convert a name to its T9 digit sequence.
  static String _nameToT9(String name) {
    final buf = StringBuffer();
    for (final c in name.toLowerCase().runes) {
      final ch = String.fromCharCode(c);
      final digit = _t9Map[ch];
      if (digit != null) buf.write(digit);
    }
    return buf.toString();
  }

  void _updateContactMatches() {
    final digits = _numberController.text.trim();
    if (digits.isEmpty) {
      setState(() => _matchingContacts = []);
      return;
    }

    final directory = ref.read(directoryProvider).valueOrNull ?? [];
    final matches = directory.where((entry) {
      // Match by extension prefix.
      if (entry.extension_.startsWith(digits)) return true;
      // Match by T9 name prefix.
      final t9 = _nameToT9(entry.name);
      if (t9.startsWith(digits)) return true;
      // Match by T9 against each word in the name.
      for (final word in entry.name.split(RegExp(r'\s+'))) {
        if (_nameToT9(word).startsWith(digits)) return true;
      }
      return false;
    }).take(5).toList();

    setState(() => _matchingContacts = matches);
  }

  void _selectContact(DirectoryEntry entry) {
    _numberController.text = entry.extension_;
    _numberController.selection = TextSelection.fromPosition(
      TextPosition(offset: _numberController.text.length),
    );
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
        SnackBar(content: Text('Call failed: ${formatError(e)}')),
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
      body: SafeArea(
        child: Column(
          children: [
            // SIP status bar at top.
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: Dimensions.space16,
                vertical: Dimensions.space8,
              ),
              child: const Align(
                alignment: Alignment.centerRight,
                child: SipStatusIndicator(),
              ),
            ),
            const SizedBox(height: Dimensions.space8),
            // Number display.
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: Dimensions.space32),
              child: TextField(
                controller: _numberController,
                textAlign: TextAlign.center,
                style: AppTypography.mono(
                  fontSize: 32,
                  fontWeight: FontWeight.w500,
                  color: colorScheme.onSurface,
                  letterSpacing: 2,
                ),
                decoration: InputDecoration(
                  border: InputBorder.none,
                  enabledBorder: InputBorder.none,
                  focusedBorder: InputBorder.none,
                  hintText: 'Enter number',
                  hintStyle: AppTypography.mono(
                    fontSize: 32,
                    fontWeight: FontWeight.w400,
                    color: colorScheme.onSurfaceVariant.withOpacity(0.4),
                    letterSpacing: 2,
                  ),
                  filled: false,
                ),
                keyboardType: TextInputType.none,
                showCursor: true,
                readOnly: true,
              ),
            ),
            // Contact matches.
            if (_matchingContacts.isNotEmpty)
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 160),
                child: ListView.builder(
                  shrinkWrap: true,
                  padding:
                      const EdgeInsets.symmetric(horizontal: Dimensions.space16),
                  itemCount: _matchingContacts.length,
                  itemBuilder: (context, index) {
                    final entry = _matchingContacts[index];
                    return _ContactMatchTile(
                      entry: entry,
                      onTap: () => _selectContact(entry),
                    )
                        .animate()
                        .fadeIn(
                          duration: 200.ms,
                          delay: (30 * index).ms,
                        )
                        .slideX(
                          begin: 0.1,
                          end: 0,
                          duration: 200.ms,
                          delay: (30 * index).ms,
                          curve: Curves.easeOut,
                        );
                  },
                ),
              )
            else
              const SizedBox(height: Dimensions.space16),
            // Dialpad grid.
            Expanded(
              child: Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: Dimensions.space40),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _buildDialpadRow(['1', '2', '3']),
                    const SizedBox(height: Dimensions.space12),
                    _buildDialpadRow(['4', '5', '6']),
                    const SizedBox(height: Dimensions.space12),
                    _buildDialpadRow(['7', '8', '9']),
                    const SizedBox(height: Dimensions.space12),
                    _buildDialpadRow(['*', '0', '#']),
                    const SizedBox(height: Dimensions.space24),
                    // Action row: call button centered, backspace on right.
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        // Spacer to balance the backspace button.
                        const SizedBox(width: Dimensions.dialpadButtonSize),
                        // Call button.
                        SizedBox(
                          width: Dimensions.dialpadButtonSize,
                          height: Dimensions.dialpadButtonSize,
                          child: FilledButton(
                            onPressed: (hasActiveCall || _isPlacingCall)
                                ? null
                                : _placeCall,
                            style: FilledButton.styleFrom(
                              backgroundColor: ColorTokens.callGreen,
                              disabledBackgroundColor:
                                  ColorTokens.callGreen.withOpacity(0.3),
                              shape: const CircleBorder(),
                              padding: EdgeInsets.zero,
                              minimumSize: Size.zero,
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
                          width: Dimensions.dialpadButtonSize,
                          height: Dimensions.dialpadButtonSize,
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
      children: digits
          .map((d) => _DialpadButton(
                digit: d,
                subtitle: _subtitleFor(d),
                onTap: () => _appendDigit(d),
              ))
          .toList(),
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

class _ContactMatchTile extends StatelessWidget {
  final DirectoryEntry entry;
  final VoidCallback onTap;

  const _ContactMatchTile({required this.entry, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return ListTile(
      dense: true,
      visualDensity: VisualDensity.compact,
      leading: GradientAvatar(name: entry.name, radius: Dimensions.avatarRadiusSmall),
      title: Text(entry.name, style: const TextStyle(fontSize: 14)),
      subtitle: Text(
        'Ext. ${entry.extension_}',
        style: AppTypography.mono(
          fontSize: 12,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
      onTap: onTap,
    );
  }
}

/// Frosted glass dialpad button with mono font digit.
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      width: Dimensions.dialpadButtonSize,
      height: Dimensions.dialpadButtonSize,
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
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    digit,
                    style: AppTypography.mono(
                      fontSize: 24,
                      fontWeight: FontWeight.w500,
                      color: colorScheme.onSurface,
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
      ),
    );
  }
}
