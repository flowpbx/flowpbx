package com.flowpbx.mobile

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val channelName = "com.flowpbx.mobile/audio_session"
    private var audioFocusRequest: AudioFocusRequest? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "configure" -> {
                        configureAudioForVoip()
                        result.success(true)
                    }
                    "activate" -> {
                        val granted = requestAudioFocus()
                        result.success(granted)
                    }
                    "deactivate" -> {
                        abandonAudioFocus()
                        result.success(true)
                    }
                    "setSpeaker" -> {
                        val enabled = call.argument<Boolean>("enabled")
                        if (enabled != null) {
                            setSpeakerphone(enabled)
                            result.success(true)
                        } else {
                            result.error("INVALID_ARGS", "Missing 'enabled' argument", null)
                        }
                    }
                    else -> result.notImplemented()
                }
            }
    }

    /// Set AudioManager to MODE_IN_COMMUNICATION for VoIP.
    private fun configureAudioForVoip() {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        audioManager.mode = AudioManager.MODE_IN_COMMUNICATION
    }

    /// Request audio focus for a VoIP call with AUDIOFOCUS_GAIN.
    /// Uses AudioFocusRequest (API 26+) — our minSdk is 29.
    private fun requestAudioFocus(): Boolean {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        audioManager.mode = AudioManager.MODE_IN_COMMUNICATION

        val attrs = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
            .build()

        val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(attrs)
            .setAcceptsDelayedFocusGain(false)
            .setWillPauseWhenDucked(false)
            .build()

        audioFocusRequest = request
        val result = audioManager.requestAudioFocus(request)
        return result == AudioManager.AUDIOFOCUS_REQUEST_GRANTED
    }

    /// Abandon audio focus and reset audio mode when the call ends.
    private fun abandonAudioFocus() {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        audioFocusRequest?.let {
            audioManager.abandonAudioFocusRequest(it)
            audioFocusRequest = null
        }
        audioManager.mode = AudioManager.MODE_NORMAL
        audioManager.isSpeakerphoneOn = false
    }

    /// Toggle speakerphone on or off.
    private fun setSpeakerphone(enabled: Boolean) {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        audioManager.isSpeakerphoneOn = enabled
    }
}
