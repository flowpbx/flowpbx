import 'package:flutter_ringtone_player/flutter_ringtone_player.dart';

/// Manages ringtone playback for incoming calls.
class RingtoneService {
  final _player = FlutterRingtonePlayer();
  bool _isPlaying = false;

  bool get isPlaying => _isPlaying;

  /// Start playing the device's default ringtone in a loop.
  void startRinging() {
    if (_isPlaying) return;
    _isPlaying = true;
    _player.play(
      android: AndroidSounds.ringtone,
      ios: IosSounds.electronic,
      looping: true,
      volume: 1.0,
    );
  }

  /// Stop the ringtone.
  void stopRinging() {
    if (!_isPlaying) return;
    _isPlaying = false;
    _player.stop();
  }
}
