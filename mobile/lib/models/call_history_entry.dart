/// A call history entry from the PBX CDR records.
class CallHistoryEntry {
  final int id;
  final String callId;
  final DateTime startTime;
  final DateTime? answerTime;
  final DateTime? endTime;
  final int? duration;
  final String callerIdName;
  final String callerIdNum;
  final String callee;
  final String direction;
  final String disposition;

  const CallHistoryEntry({
    required this.id,
    required this.callId,
    required this.startTime,
    this.answerTime,
    this.endTime,
    this.duration,
    required this.callerIdName,
    required this.callerIdNum,
    required this.callee,
    required this.direction,
    required this.disposition,
  });

  factory CallHistoryEntry.fromJson(Map<String, dynamic> json) {
    return CallHistoryEntry(
      id: json['id'] as int,
      callId: json['call_id'] as String,
      startTime: DateTime.parse(json['start_time'] as String),
      answerTime: json['answer_time'] != null
          ? DateTime.parse(json['answer_time'] as String)
          : null,
      endTime: json['end_time'] != null
          ? DateTime.parse(json['end_time'] as String)
          : null,
      duration: json['duration'] as int?,
      callerIdName: json['caller_id_name'] as String? ?? '',
      callerIdNum: json['caller_id_num'] as String? ?? '',
      callee: json['callee'] as String? ?? '',
      direction: json['direction'] as String? ?? '',
      disposition: json['disposition'] as String? ?? '',
    );
  }

  /// Whether this call was answered.
  bool get isAnswered => disposition == 'answered';

  /// Whether this call was missed (no answer or cancelled by caller).
  bool get isMissed =>
      disposition == 'no_answer' || disposition == 'cancelled';

  /// Whether this was an inbound call (from trunk or another extension).
  bool get isInbound => direction == 'inbound' || direction == 'internal';

  /// Whether this was an outbound call.
  bool get isOutbound => direction == 'outbound';

  /// The display name of the remote party.
  String get remoteName {
    if (isOutbound) return callee;
    if (callerIdName.isNotEmpty) return callerIdName;
    return callerIdNum;
  }

  /// The number of the remote party.
  String get remoteNumber {
    if (isOutbound) return callee;
    return callerIdNum;
  }
}
