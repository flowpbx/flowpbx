import 'dart:io';

import 'package:flutter/services.dart';

/// Manages the iOS Call Directory extension for caller ID identification.
///
/// Writes PBX contact entries to shared App Group storage, then triggers
/// a reload of the Call Directory extension so iOS can identify incoming
/// callers by name.
///
/// On Android, all methods are no-ops.
class CallDirectoryService {
  static const _channel =
      MethodChannel('com.flowpbx.mobile/call_directory');

  /// Update caller ID entries in the shared App Group storage and reload
  /// the Call Directory extension.
  ///
  /// Each entry should contain a phone number (as Int64 with country code)
  /// and a display label. Entries must be sorted by phone number ascending.
  ///
  /// [entries] is a list of maps with keys: "number" (int) and "label" (String).
  Future<bool> updateEntries(List<Map<String, dynamic>> entries) async {
    if (!Platform.isIOS) return false;

    try {
      final result =
          await _channel.invokeMethod('updateEntries', entries);
      return result == true;
    } on PlatformException {
      return false;
    }
  }

  /// Reload the Call Directory extension so iOS picks up new entries.
  Future<bool> reloadExtension() async {
    if (!Platform.isIOS) return false;

    try {
      final result = await _channel.invokeMethod('reloadExtension');
      return result == true;
    } on PlatformException {
      return false;
    }
  }

  /// Check whether the user has enabled the Call Directory extension
  /// in Settings > Phone > Call Blocking & Identification.
  Future<bool> isEnabled() async {
    if (!Platform.isIOS) return false;

    try {
      final result =
          await _channel.invokeMethod<bool>('getEnabledStatus');
      return result ?? false;
    } on PlatformException {
      return false;
    }
  }

  /// Convenience method: convert a list of directory entries (extension + name)
  /// into the format expected by the Call Directory extension.
  ///
  /// PBX extensions are internal numbers (e.g. "100", "201"). The Call
  /// Directory extension needs full E.164 phone numbers (Int64). Since PBX
  /// extensions are not real phone numbers, we prefix them with a country
  /// code placeholder so iOS can match them when the PBX presents the
  /// caller ID on incoming calls.
  ///
  /// [countryCode] is the numeric country calling code (e.g. 1 for US).
  static List<Map<String, dynamic>> formatEntries({
    required List<({String number, String label})> contacts,
    int countryCode = 1,
  }) {
    final entries = <Map<String, dynamic>>[];

    for (final contact in contacts) {
      final digits = contact.number.replaceAll(RegExp(r'[^\d]'), '');
      if (digits.isEmpty) continue;

      // Build the full number with country code.
      final fullNumber = '$countryCode$digits';
      final number = int.tryParse(fullNumber);
      if (number == null) continue;

      entries.add({
        'number': number,
        'label': 'FlowPBX: ${contact.label}',
      });
    }

    // Entries must be sorted by phone number ascending for iOS.
    entries.sort(
      (a, b) => (a['number'] as int).compareTo(b['number'] as int),
    );

    return entries;
  }
}
