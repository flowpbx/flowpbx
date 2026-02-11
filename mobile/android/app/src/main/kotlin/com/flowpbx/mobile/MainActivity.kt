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
import android.net.Uri
import android.os.PowerManager
import android.provider.Settings
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

        // Battery optimization platform channel.
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "com.flowpbx.mobile/battery_optimization")
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "isIgnoringBatteryOptimizations" -> {
                        result.success(isIgnoringBatteryOptimizations())
                    }
                    "requestIgnoreBatteryOptimizations" -> {
                        result.success(requestIgnoreBatteryOptimizations())
                    }
                    "openBatteryOptimizationSettings" -> {
                        result.success(openBatteryOptimizationSettings())
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
    /// Includes a focus change listener so we can hold/resume the call
    /// when another app temporarily takes audio focus.
    private fun requestAudioFocus(): Boolean {
        val audioManager = getSystemService(Context.AUDIO_SERVICE) as AudioManager
        audioManager.mode = AudioManager.MODE_IN_COMMUNICATION

        val attrs = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
            .build()

        val focusChangeListener = AudioManager.OnAudioFocusChangeListener { focusChange ->
            when (focusChange) {
                AudioManager.AUDIOFOCUS_LOSS,
                AudioManager.AUDIOFOCUS_LOSS_TRANSIENT -> {
                    methodChannel?.invokeMethod("onAudioInterruption", "began")
                }
                AudioManager.AUDIOFOCUS_GAIN -> {
                    // Regained focus — re-set communication mode and notify Dart.
                    audioManager.mode = AudioManager.MODE_IN_COMMUNICATION
                    methodChannel?.invokeMethod("onAudioInterruption", "ended")
                }
                AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK -> {
                    // Another app wants to play briefly (notification sound).
                    // VoIP calls should not duck — notify Dart of transient loss.
                    methodChannel?.invokeMethod("onAudioInterruption", "focusLost")
                }
            }
        }

        val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
            .setAudioAttributes(attrs)
            .setAcceptsDelayedFocusGain(false)
            .setWillPauseWhenDucked(false)
            .setOnAudioFocusChangeListener(focusChangeListener)
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

    /// Check if the app is exempt from battery optimization (Doze).
    private fun isIgnoringBatteryOptimizations(): Boolean {
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        return powerManager.isIgnoringBatteryOptimizations(packageName)
    }

    /// Request the system to whitelist this app from battery optimization.
    /// Launches ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS which shows a
    /// system confirmation dialog.
    @Suppress("BatteryLife")
    private fun requestIgnoreBatteryOptimizations(): Boolean {
        return try {
            val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
                data = Uri.parse("package:$packageName")
            }
            startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }

    /// Open the general battery optimization settings page as a fallback.
    private fun openBatteryOptimizationSettings(): Boolean {
        return try {
            val intent = Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)
            startActivity(intent)
            true
        } catch (_: Exception) {
            false
        }
    }
}
