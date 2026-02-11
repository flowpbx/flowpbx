import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/directory_entry.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';

/// Fetches the PBX extension directory. Refreshable via ref.invalidate().
final directoryProvider = FutureProvider<List<DirectoryEntry>>((ref) async {
  final api = ref.watch(apiServiceProvider);
  final data = await api.getDirectory();
  return data
      .map((e) => DirectoryEntry.fromJson(e as Map<String, dynamic>))
      .toList()
    ..sort((a, b) => a.extension_.compareTo(b.extension_));
});
