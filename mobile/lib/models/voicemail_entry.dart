/// A voicemail message from the PBX.
class VoicemailEntry {
  final int id;
  final int mailboxId;
  final String callerIdName;
  final String callerIdNum;
  final DateTime timestamp;
  final int duration;
  final bool read;
  final DateTime? readAt;
  final String transcription;
  final DateTime createdAt;

  const VoicemailEntry({
    required this.id,
    required this.mailboxId,
    required this.callerIdName,
    required this.callerIdNum,
    required this.timestamp,
    required this.duration,
    required this.read,
    this.readAt,
    required this.transcription,
    required this.createdAt,
  });

  factory VoicemailEntry.fromJson(Map<String, dynamic> json) {
    return VoicemailEntry(
      id: json['id'] as int,
      mailboxId: json['mailbox_id'] as int,
      callerIdName: json['caller_id_name'] as String? ?? '',
      callerIdNum: json['caller_id_num'] as String? ?? '',
      timestamp: DateTime.parse(json['timestamp'] as String),
      duration: json['duration'] as int? ?? 0,
      read: json['read'] as bool? ?? false,
      readAt: json['read_at'] != null
          ? DateTime.parse(json['read_at'] as String)
          : null,
      transcription: json['transcription'] as String? ?? '',
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// Whether this voicemail is unread.
  bool get isUnread => !read;

  /// Display name for the caller.
  String get callerName {
    if (callerIdName.isNotEmpty) return callerIdName;
    if (callerIdNum.isNotEmpty) return callerIdNum;
    return 'Unknown';
  }
}
