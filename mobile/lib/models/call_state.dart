/// Represents the state of an active call.
enum CallStatus {
  idle,
  dialing,
  ringing,
  connected,
  holding,
  held,
  disconnecting,
}

/// Active call state for UI consumption.
class ActiveCallState {
  final int? callId;
  final CallStatus status;
  final String remoteNumber;
  final String? remoteDisplayName;
  final bool isMuted;
  final bool isSpeaker;
  final bool isHeld;
  final bool isIncoming;
  final DateTime? connectedAt;
  final String? error;

  const ActiveCallState({
    this.callId,
    this.status = CallStatus.idle,
    this.remoteNumber = '',
    this.remoteDisplayName,
    this.isMuted = false,
    this.isSpeaker = false,
    this.isHeld = false,
    this.isIncoming = false,
    this.connectedAt,
    this.error,
  });

  bool get isActive => status != CallStatus.idle;

  ActiveCallState copyWith({
    int? callId,
    CallStatus? status,
    String? remoteNumber,
    String? remoteDisplayName,
    bool? isMuted,
    bool? isSpeaker,
    bool? isHeld,
    bool? isIncoming,
    DateTime? connectedAt,
    String? error,
  }) {
    return ActiveCallState(
      callId: callId ?? this.callId,
      status: status ?? this.status,
      remoteNumber: remoteNumber ?? this.remoteNumber,
      remoteDisplayName: remoteDisplayName ?? this.remoteDisplayName,
      isMuted: isMuted ?? this.isMuted,
      isSpeaker: isSpeaker ?? this.isSpeaker,
      isHeld: isHeld ?? this.isHeld,
      isIncoming: isIncoming ?? this.isIncoming,
      connectedAt: connectedAt ?? this.connectedAt,
      error: error,
    );
  }

  static const idle = ActiveCallState();
}
