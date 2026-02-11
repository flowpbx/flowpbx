import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flowpbx_mobile/models/extension_profile.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';

final profileProvider =
    AsyncNotifierProvider<ProfileNotifier, ExtensionProfile?>(
        ProfileNotifier.new);

class ProfileNotifier extends AsyncNotifier<ExtensionProfile?> {
  @override
  Future<ExtensionProfile?> build() async {
    final auth = ref.watch(authStateProvider);
    final isAuthenticated = auth.valueOrNull?.isAuthenticated ?? false;
    if (!isAuthenticated) return null;

    final api = ref.read(apiServiceProvider);
    final data = await api.getProfile();
    return ExtensionProfile.fromJson(data);
  }

  /// Toggle DND on the PBX and refresh the local profile.
  Future<void> toggleDnd(bool enabled) async {
    final api = ref.read(apiServiceProvider);
    final data = await api.updateProfile({'dnd': enabled});
    state = AsyncData(ExtensionProfile.fromJson(data));
  }

  /// Toggle follow-me on the PBX and refresh the local profile.
  Future<void> toggleFollowMe(bool enabled) async {
    final api = ref.read(apiServiceProvider);
    final data = await api.updateProfile({'follow_me_enabled': enabled});
    state = AsyncData(ExtensionProfile.fromJson(data));
  }

  /// Refresh the profile from the PBX.
  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() async {
      final api = ref.read(apiServiceProvider);
      final data = await api.getProfile();
      return ExtensionProfile.fromJson(data);
    });
  }
}
