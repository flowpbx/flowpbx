import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:just_audio/just_audio.dart';
import 'package:flowpbx_mobile/providers/auth_provider.dart';

/// State for the voicemail audio player.
class VoicemailPlayerState {
  final int? currentId;
  final bool isPlaying;
  final Duration position;
  final Duration duration;
  final double speed;
  final bool isLoading;

  const VoicemailPlayerState({
    this.currentId,
    this.isPlaying = false,
    this.position = Duration.zero,
    this.duration = Duration.zero,
    this.speed = 1.0,
    this.isLoading = false,
  });

  VoicemailPlayerState copyWith({
    int? currentId,
    bool? isPlaying,
    Duration? position,
    Duration? duration,
    double? speed,
    bool? isLoading,
  }) {
    return VoicemailPlayerState(
      currentId: currentId ?? this.currentId,
      isPlaying: isPlaying ?? this.isPlaying,
      position: position ?? this.position,
      duration: duration ?? this.duration,
      speed: speed ?? this.speed,
      isLoading: isLoading ?? this.isLoading,
    );
  }

  static const empty = VoicemailPlayerState();
}

/// Manages voicemail audio playback using just_audio.
final voicemailPlayerProvider =
    NotifierProvider<VoicemailPlayerNotifier, VoicemailPlayerState>(
  VoicemailPlayerNotifier.new,
);

class VoicemailPlayerNotifier extends Notifier<VoicemailPlayerState> {
  AudioPlayer? _player;

  @override
  VoicemailPlayerState build() {
    ref.onDispose(() {
      _player?.dispose();
      _player = null;
    });
    return VoicemailPlayerState.empty;
  }

  AudioPlayer _ensurePlayer() {
    if (_player == null) {
      _player = AudioPlayer();
      _player!.positionStream.listen((pos) {
        if (state.currentId != null) {
          state = state.copyWith(position: pos);
        }
      });
      _player!.durationStream.listen((dur) {
        if (dur != null && state.currentId != null) {
          state = state.copyWith(duration: dur);
        }
      });
      _player!.playerStateStream.listen((playerState) {
        if (state.currentId == null) return;
        final playing = playerState.playing;
        final processingState = playerState.processingState;
        if (processingState == ProcessingState.completed) {
          // Reset to beginning when playback completes.
          state = state.copyWith(isPlaying: false, position: Duration.zero);
          _player?.seek(Duration.zero);
          _player?.pause();
        } else {
          state = state.copyWith(
            isPlaying: playing && processingState == ProcessingState.ready,
            isLoading: processingState == ProcessingState.loading ||
                processingState == ProcessingState.buffering,
          );
        }
      });
    }
    return _player!;
  }

  /// Start playing a voicemail by ID, or toggle pause if already playing.
  Future<void> play(int voicemailId) async {
    final player = _ensurePlayer();

    if (state.currentId == voicemailId) {
      // Same voicemail — toggle play/pause.
      if (player.playing) {
        await player.pause();
      } else {
        await player.play();
      }
      return;
    }

    // Different voicemail — load and play.
    state = state.copyWith(
      currentId: voicemailId,
      isPlaying: false,
      isLoading: true,
      position: Duration.zero,
      duration: Duration.zero,
    );

    final api = ref.read(apiServiceProvider);
    final token = await api.getAuthToken();
    final url = api.voicemailAudioUrl(voicemailId);

    await player.setAudioSource(
      AudioSource.uri(
        Uri.parse(url),
        headers: {
          if (token != null) 'Authorization': 'Bearer $token',
        },
      ),
    );
    await player.setSpeed(state.speed);
    await player.play();
  }

  /// Pause playback.
  Future<void> pause() async {
    await _player?.pause();
  }

  /// Seek to a position.
  Future<void> seek(Duration position) async {
    await _player?.seek(position);
  }

  /// Cycle playback speed: 1x → 1.5x → 2x → 1x.
  Future<void> cycleSpeed() async {
    double nextSpeed;
    if (state.speed < 1.25) {
      nextSpeed = 1.5;
    } else if (state.speed < 1.75) {
      nextSpeed = 2.0;
    } else {
      nextSpeed = 1.0;
    }
    state = state.copyWith(speed: nextSpeed);
    await _player?.setSpeed(nextSpeed);
  }

  /// Stop and clear the current playback.
  Future<void> stop() async {
    await _player?.stop();
    state = VoicemailPlayerState.empty;
  }
}
