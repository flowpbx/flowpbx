import 'package:flutter/material.dart';
import 'package:flowpbx_mobile/services/app_error.dart';

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
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.error_outline,
              size: 48,
              color: colorScheme.error,
            ),
            const SizedBox(height: 16),
            Text(
              fallbackMessage,
              style: Theme.of(context).textTheme.bodyLarge,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              message,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: colorScheme.onSurfaceVariant,
                  ),
              textAlign: TextAlign.center,
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: onRetry,
                child: const Text('Retry'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
