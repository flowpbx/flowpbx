import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flowpbx_mobile/services/app_error.dart';
import 'package:flowpbx_mobile/theme/dimensions.dart';

/// Reusable full-screen error state with icon, user-friendly message, and retry
/// button.  Used by list screens (voicemail, call history, contacts, settings)
/// in their `AsyncValue.when(error: ...)` branches.
class ErrorBanner extends StatelessWidget {
  final Object error;
  final String fallbackMessage;
  final VoidCallback? onRetry;

  const ErrorBanner({
    super.key,
    required this.error,
    this.fallbackMessage = 'Something went wrong',
    this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final message = formatError(error);

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(Dimensions.space24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.error_outline,
              size: 48,
              color: colorScheme.error,
            ),
            const SizedBox(height: Dimensions.space16),
            Text(
              fallbackMessage,
              style: Theme.of(context).textTheme.bodyLarge,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: Dimensions.space8),
            Text(
              message,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
              textAlign: TextAlign.center,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: Dimensions.space16),
              FilledButton.tonal(
                onPressed: onRetry,
                child: const Text('Retry'),
              ),
            ],
          ],
        )
            .animate()
            .fadeIn(duration: 400.ms)
            .shake(hz: 2, offset: const Offset(2, 0), duration: 400.ms),
      ),
    );
  }
}
