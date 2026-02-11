package com.flowpbx.mobile

import android.net.Uri
import android.os.Bundle
import android.telecom.Connection
import android.telecom.ConnectionRequest
import android.telecom.ConnectionService
import android.telecom.DisconnectCause
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager

/**
 * Android Telecom ConnectionService for native call integration.
 *
 * This is the Android equivalent of iOS CallKit. It allows the system
 * to manage calls via the native dialer UI, lock screen, and Bluetooth
 * headset controls.
 */
class FlowPBXConnectionService : ConnectionService() {

    companion object {
        /** Active connections keyed by call UUID. */
        val connections = mutableMapOf<String, FlowPBXConnection>()

        /** Reference to the method channel handler for sending events to Flutter. */
        var channelHandler: ConnectionServiceChannelHandler? = null

        /** Application context for dismissing notifications from Connection callbacks. */
        var appContext: android.content.Context? = null
    }

    override fun onCreateIncomingConnection(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ): Connection {
        appContext = applicationContext

        val extras = request?.extras ?: Bundle()
        val uuid = extras.getString("uuid") ?: ""
        val handle = extras.getString("handle") ?: ""
        val displayName = extras.getString("displayName")

        val connection = FlowPBXConnection(uuid).apply {
            setAddress(
                Uri.fromParts("tel", handle, null),
                TelecomManager.PRESENTATION_ALLOWED
            )
            if (!displayName.isNullOrEmpty()) {
                setCallerDisplayName(displayName, TelecomManager.PRESENTATION_ALLOWED)
            }
            setInitializing()
            setRinging()
            connectionCapabilities = Connection.CAPABILITY_HOLD or
                Connection.CAPABILITY_SUPPORT_HOLD or
                Connection.CAPABILITY_MUTE
            audioModeIsVoip = true
        }

        connections[uuid] = connection

        // Show heads-up notification with Answer/Reject for self-managed calls.
        IncomingCallNotificationHelper.show(
            applicationContext, uuid, handle, displayName
        )

        return connection
    }

    override fun onCreateIncomingConnectionFailed(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ) {
        val uuid = request?.extras?.getString("uuid") ?: return
        channelHandler?.onConnectionEvent("onConnectionFailed", mapOf("uuid" to uuid))
    }

    override fun onCreateOutgoingConnection(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ): Connection {
        appContext = applicationContext
        val extras = request?.extras ?: Bundle()
        val uuid = extras.getString("uuid") ?: ""
        val handle = request?.address?.schemeSpecificPart ?: ""

        val connection = FlowPBXConnection(uuid).apply {
            setAddress(
                request?.address ?: Uri.fromParts("tel", handle, null),
                TelecomManager.PRESENTATION_ALLOWED
            )
            setInitializing()
            setDialing()
            connectionCapabilities = Connection.CAPABILITY_HOLD or
                Connection.CAPABILITY_SUPPORT_HOLD or
                Connection.CAPABILITY_MUTE
            audioModeIsVoip = true
        }

        connections[uuid] = connection
        return connection
    }

    override fun onCreateOutgoingConnectionFailed(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?
    ) {
        val uuid = request?.extras?.getString("uuid") ?: return
        channelHandler?.onConnectionEvent("onConnectionFailed", mapOf("uuid" to uuid))
    }
}
