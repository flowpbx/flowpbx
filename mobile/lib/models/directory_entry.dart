/// A lightweight contact entry from the PBX extension directory.
class DirectoryEntry {
  final int id;
  final String extension_;
  final String name;
  final bool online;

  const DirectoryEntry({
    required this.id,
    required this.extension_,
    required this.name,
    required this.online,
  });

  factory DirectoryEntry.fromJson(Map<String, dynamic> json) {
    return DirectoryEntry(
      id: json['id'] as int,
      extension_: json['extension'] as String,
      name: json['name'] as String,
      online: json['online'] as bool? ?? false,
    );
  }
}
