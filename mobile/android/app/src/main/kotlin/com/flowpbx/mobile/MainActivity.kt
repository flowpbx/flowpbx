package com.flowpbx.mobile

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothProfile
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.media.AudioAttributes
import android.media.AudioDeviceInfo
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.PowerManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val channelName = "com.flowpbx.mobile/audio_session"
    private var audioFocusRequest: AudioFocusRequest? = null
    private var methodChannel: MethodChannel? = null
    private var headsetReceiver: BroadcastReceiver? = null
    private var proximityWakeLock: PowerManager.WakeLock? = null
    private var connectionServiceHandler: ConnectionServiceChannelHandler? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        val channel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
        methodChannel = channel

        channel.setMethodCallHandler { call, result ->
            when (call.method) {
                "configure" -> {
                    configureAudioForVoip()
                    result.success(true)
                }
                "activate" -> {
                    val granted = requestAudioFocus()
                    registerHeadsetReceiver()
                    result.success(granted)
                }
                "deactivate" -> {
                    abandonAudioFocus()
                    unregisterHeadsetReceiver()
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
                "getAudioRoute" -> {
                    result.success(currentAudioRoute())
                }
                else -> result.notImplemented()
            }
        }

        // Proximity sensor platform channel.
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "com.flowpbx.mobile/proximity")
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "enable" -> {
                        enableProximitySensor()
                        result.success(true)
                    }
                    "disable" -> {
                        disableProximitySensor()
                        result.success(true)
                    }
                    else -> result.notImplemented()
                }
            }

        // ConnectionService platform channel (Android equivalent of iOS CallKit).
        val connectionChannel = MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            "com.flowpbx.mobile/connection"
        )
        connectionServiceHandler = ConnectionServiceChannelHandler(this, connectionChannel)
        connectionChannel.setMethodCallHandler(connectionServiceHandler)
    }

    override fun onDestroy() {
        unregisterHeadsetReceiver()
        disableProximitySensor()
        connectionServiceHandler?.dispose()
        connectionServiceHandler = null
        super.onDestroy()
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

    /// Determine the current audio output route.
    private fun currentAudioRoute(): String {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager

        // Check active communication devices (API 31+, our minSdk is 29).
        val devices = audioManager.getDevices(AudioManager.GET_DEVICES_OUTPUTS)
        val hasBluetooth = devices.any {
            it.type == AudioDeviceInfo.TYPE_BLUETOOTH_SCO ||
            it.type == AudioDeviceInfo.TYPE_BLUETOOTH_A2DP
        }
        val hasWiredHeadset = devices.any {
            it.type == AudioDeviceInfo.TYPE_WIRED_HEADSET ||
            it.type == AudioDeviceInfo.TYPE_WIRED_HEADPHONES ||
            it.type == AudioDeviceInfo.TYPE_USB_HEADSET
        }

        if (audioManager.isSpeakerphoneOn) {
            return "speaker"
        }
        if (audioManager.isBluetoothScoOn || (hasBluetooth && isBluetoothAudioConnected())) {
            return "bluetooth"
        }
        if (audioManager.isWiredHeadsetOn || hasWiredHeadset) {
            return "headset"
        }
        return "earpiece"
    }

    /// Check if a Bluetooth audio device is actively connected.
    private fun isBluetoothAudioConnected(): Boolean {
        return try {
            val adapter = BluetoothAdapter.getDefaultAdapter() ?: return false
            adapter.getProfileConnectionState(BluetoothProfile.HEADSET) == BluetoothProfile.STATE_CONNECTED
        } catch (_: SecurityException) {
            false
        }
    }

    /// Register a broadcast receiver for headset and Bluetooth state changes.
    private fun registerHeadsetReceiver() {
        if (headsetReceiver != null) return

        headsetReceiver = object : BroadcastReceiver() {
            override fun onReceive(context: Context?, intent: Intent?) {
                val route = currentAudioRoute()
                methodChannel?.invokeMethod("onAudioRouteChanged", route)
            }
        }

        val filter = IntentFilter().apply {
            addAction(AudioManager.ACTION_HEADSET_PLUG)
            addAction(BluetoothAdapter.ACTION_CONNECTION_STATE_CHANGED)
            addAction(AudioManager.ACTION_SCO_AUDIO_STATE_UPDATED)
        }
        registerReceiver(headsetReceiver, filter)
    }

    /// Unregister the headset/Bluetooth broadcast receiver.
    private fun unregisterHeadsetReceiver() {
        headsetReceiver?.let {
            try {
                unregisterReceiver(it)
            } catch (_: IllegalArgumentException) {
                // Already unregistered.
            }
            headsetReceiver = null
        }
    }

    /// Acquire a proximity wake lock to turn screen off when near ear.
    @Suppress("DEPRECATION")
    private fun enableProximitySensor() {
        if (proximityWakeLock?.isHeld == true) return

        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        proximityWakeLock = powerManager.newWakeLock(
            PowerManager.PROXIMITY_SCREEN_OFF_WAKE_LOCK,
            "flowpbx:proximity"
        )
        proximityWakeLock?.acquire()
    }

    /// Release the proximity wake lock.
    private fun disableProximitySensor() {
        proximityWakeLock?.let {
            if (it.isHeld) {
                it.release()
            }
        }
        proximityWakeLock = null
    }
}
