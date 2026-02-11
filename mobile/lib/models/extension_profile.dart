/// Extension profile returned from the PBX.
class ExtensionProfile {
  final int id;
  final String extension_;
  final String name;
  final String email;
  final bool dnd;
  final bool followMeEnabled;
  final List<String> followMeNumbers;
  final String followMeStrategy;
  final bool followMeConfirm;
  final DateTime updatedAt;

  const ExtensionProfile({
    required this.id,
    required this.extension_,
    required this.name,
    required this.email,
    required this.dnd,
    required this.followMeEnabled,
    required this.followMeNumbers,
    required this.followMeStrategy,
    required this.followMeConfirm,
    required this.updatedAt,
  });

  factory ExtensionProfile.fromJson(Map<String, dynamic> json) {
    return ExtensionProfile(
      id: json['id'] as int,
      extension_: json['extension'] as String,
      name: json['name'] as String,
      email: json['email'] as String? ?? '',
      dnd: json['dnd'] as bool? ?? false,
      followMeEnabled: json['follow_me_enabled'] as bool? ?? false,
      followMeNumbers: (json['follow_me_numbers'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      followMeStrategy: json['follow_me_strategy'] as String? ?? 'ring_all',
      followMeConfirm: json['follow_me_confirm'] as bool? ?? false,
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }
}
