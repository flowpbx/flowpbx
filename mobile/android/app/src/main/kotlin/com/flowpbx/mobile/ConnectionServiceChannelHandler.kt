package com.flowpbx.mobile

import android.Manifest
import android.app.Activity
import android.content.ComponentName
import android.content.Context
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.telecom.PhoneAccount
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

/**
 * Handles Flutter method channel calls for Android ConnectionService integration.
 *
 * This is the Android equivalent of the iOS CallKitManager — it bridges between
 * the Flutter Dart side and the native Android Telecom framework.
 */
class ConnectionServiceChannelHandler(
    private val context: Context,
    private val channel: MethodChannel
) : MethodChannel.MethodCallHandler {

    private val telecomManager: TelecomManager
        get() = context.getSystemService(Context.TELECOM_SERVICE) as TelecomManager

    private val phoneAccountHandle: PhoneAccountHandle by lazy {
        PhoneAccountHandle(
            ComponentName(context, FlowPBXConnectionService::class.java),
            "flowpbx_account"
        )
    }

    init {
        // Register the PhoneAccount with the system so we can place/receive calls.
        registerPhoneAccount()
        // Wire ourselves as the channel handler for ConnectionService events.
        FlowPBXConnectionService.channelHandler = this
        // Pre-create the notification channel and request POST_NOTIFICATIONS on API 33+.
        IncomingCallNotificationHelper.createChannel(context)
        requestNotificationPermission()
    }

    private fun registerPhoneAccount() {
        val account = PhoneAccount.builder(phoneAccountHandle, "FlowPBX")
            .setCapabilities(
                PhoneAccount.CAPABILITY_CALL_PROVIDER or
                PhoneAccount.CAPABILITY_SELF_MANAGED
            )
            .setSupportedUriSchemes(listOf(PhoneAccount.SCHEME_TEL, PhoneAccount.SCHEME_SIP))
            .build()
        telecomManager.registerPhoneAccount(account)
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "reportIncomingCall" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                val handle = call.argument<String>("handle") ?: ""
                val displayName = call.argument<String>("displayName")
                reportIncomingCall(uuid, handle, displayName, result)
            }
            "reportOutgoingCall" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                val handle = call.argument<String>("handle") ?: ""
                reportOutgoingCall(uuid, handle, result)
            }
            "reportCallConnected" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                reportCallConnected(uuid)
                result.success(true)
            }
            "reportCallEnded" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                val reason = call.argument<Int>("reason") ?: 1
                reportCallEnded(uuid, reason)
                result.success(true)
            }
            "endCall" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                endCall(uuid)
                result.success(true)
            }
            "setMuted" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                val muted = call.argument<Boolean>("muted") ?: false
                setMuted(uuid, muted)
                result.success(true)
            }
            "setHeld" -> {
                val uuid = call.argument<String>("uuid") ?: return result.error("INVALID_ARGS", "Missing uuid", null)
                val held = call.argument<Boolean>("held") ?: false
                setHeld(uuid, held)
                result.success(true)
            }
            else -> result.notImplemented()
        }
    }

    private fun reportIncomingCall(uuid: String, handle: String, displayName: String?, result: MethodChannel.Result) {
        val extras = Bundle().apply {
            putString("uuid", uuid)
            putString("handle", handle)
            if (displayName != null) putString("displayName", displayName)
            putParcelable(TelecomManager.EXTRA_PHONE_ACCOUNT_HANDLE, phoneAccountHandle)
        }
        try {
            telecomManager.addNewIncomingCall(phoneAccountHandle, extras)
            result.success(true)
        } catch (e: SecurityException) {
            result.error("PERMISSION_DENIED", "Cannot add incoming call: ${e.message}", null)
        } catch (e: Exception) {
            result.error("ERROR", "Failed to report incoming call: ${e.message}", null)
        }
    }

    private fun reportOutgoingCall(uuid: String, handle: String, result: MethodChannel.Result) {
        val extras = Bundle().apply {
            putString("uuid", uuid)
            putParcelable(TelecomManager.EXTRA_PHONE_ACCOUNT_HANDLE, phoneAccountHandle)
        }
        try {
            telecomManager.placeCall(
                Uri.fromParts("tel", handle, null),
                extras
            )
            result.success(true)
        } catch (e: SecurityException) {
            result.error("PERMISSION_DENIED", "Cannot place call: ${e.message}", null)
        } catch (e: Exception) {
            result.error("ERROR", "Failed to report outgoing call: ${e.message}", null)
        }
    }

    private fun reportCallConnected(uuid: String) {
        FlowPBXConnectionService.connections[uuid]?.setConnectionActive()
    }

    private fun reportCallEnded(uuid: String, reason: Int) {
        FlowPBXConnectionService.connections[uuid]?.setConnectionDisconnected(reason)
    }

    private fun endCall(uuid: String) {
        val connection = FlowPBXConnectionService.connections[uuid] ?: return
        connection.setConnectionDisconnected(1)
    }

    private fun setMuted(uuid: String, muted: Boolean) {
        // Muting is handled at the SIP layer; the Connection's CallAudioState
        // is read-only from the service side. We don't need to do anything here
        // beyond acknowledging the request — the SipService mutes the mic directly.
    }

    private fun setHeld(uuid: String, held: Boolean) {
        val connection = FlowPBXConnectionService.connections[uuid] ?: return
        if (held) {
            connection.setConnectionHeld()
        } else {
            connection.setConnectionActive()
        }
    }

    /** Called by FlowPBXConnection/FlowPBXConnectionService to send events to Flutter. */
    fun onConnectionEvent(method: String, args: Map<String, Any?>) {
        channel.invokeMethod(method, args)
    }

    /** Request POST_NOTIFICATIONS permission on Android 13+ (API 33). */
    private fun requestNotificationPermission() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS)
            == PackageManager.PERMISSION_GRANTED) return
        if (context is Activity) {
            ActivityCompat.requestPermissions(
                context as Activity,
                arrayOf(Manifest.permission.POST_NOTIFICATIONS),
                1001
            )
        }
    }

    fun dispose() {
        FlowPBXConnectionService.channelHandler = null
    }
}
