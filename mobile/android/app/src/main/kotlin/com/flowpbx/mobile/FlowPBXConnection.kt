package com.flowpbx.mobile

import android.os.Bundle
import android.telecom.CallAudioState
import android.telecom.Connection
import android.telecom.DisconnectCause

/**
 * Represents an individual call connection for the Android Telecom framework.
 *
 * Mirrors the role of iOS CXCall — each active call has one Connection instance.
 * User actions on the native call UI (answer, end, mute, hold, DTMF) are
 * forwarded to Flutter via the method channel.
 */
class FlowPBXConnection(val uuid: String) : Connection() {

    private val handler: ConnectionServiceChannelHandler?
        get() = FlowPBXConnectionService.channelHandler

    override fun onAnswer() {
        dismissNotification()
        setActive()
        handler?.onConnectionEvent("onConnectionAnswer", mapOf("uuid" to uuid))
    }

    override fun onReject() {
        dismissNotification()
        setDisconnected(DisconnectCause(DisconnectCause.REJECTED))
        destroy()
        FlowPBXConnectionService.connections.remove(uuid)
        handler?.onConnectionEvent("onConnectionEnd", mapOf("uuid" to uuid))
    }

    override fun onDisconnect() {
        dismissNotification()
        setDisconnected(DisconnectCause(DisconnectCause.LOCAL))
        destroy()
        FlowPBXConnectionService.connections.remove(uuid)
        handler?.onConnectionEvent("onConnectionEnd", mapOf("uuid" to uuid))
    }

    override fun onAbort() {
        dismissNotification()
        setDisconnected(DisconnectCause(DisconnectCause.CANCELED))
        destroy()
        FlowPBXConnectionService.connections.remove(uuid)
        handler?.onConnectionEvent("onConnectionEnd", mapOf("uuid" to uuid))
    }

    override fun onHold() {
        setOnHold()
        handler?.onConnectionEvent("onConnectionHold", mapOf(
            "uuid" to uuid,
            "held" to true
        ))
    }

    override fun onUnhold() {
        setActive()
        handler?.onConnectionEvent("onConnectionHold", mapOf(
            "uuid" to uuid,
            "held" to false
        ))
    }

    override fun onCallAudioStateChanged(state: CallAudioState?) {
        state ?: return
        handler?.onConnectionEvent("onConnectionMute", mapOf(
            "uuid" to uuid,
            "muted" to state.isMuted
        ))
    }

    override fun onPlayDtmfTone(c: Char) {
        handler?.onConnectionEvent("onConnectionDTMF", mapOf(
            "uuid" to uuid,
            "digits" to c.toString()
        ))
    }

    // -- Methods called from Flutter to update connection state --

    /** Mark the connection as actively connected (call answered/connected). */
    fun setConnectionActive() {
        setActive()
    }

    /** Mark the connection as on hold. */
    fun setConnectionHeld() {
        setOnHold()
    }

    /** Mark the connection as dialing (outgoing call in progress). */
    fun setConnectionDialing() {
        setDialing()
    }

    /** Mark the connection as ringing (incoming call). */
    fun setConnectionRinging() {
        setRinging()
    }

    /** Disconnect and clean up this connection. */
    fun setConnectionDisconnected(reason: Int) {
        dismissNotification()
        val cause = when (reason) {
            1 -> DisconnectCause(DisconnectCause.REMOTE)
            2 -> DisconnectCause(DisconnectCause.ERROR)
            3 -> DisconnectCause(DisconnectCause.MISSED)
            4 -> DisconnectCause(DisconnectCause.REJECTED)
            else -> DisconnectCause(DisconnectCause.UNKNOWN)
        }
        setDisconnected(cause)
        destroy()
        FlowPBXConnectionService.connections.remove(uuid)
    }

    /** Dismiss the incoming call heads-up notification if visible. */
    private fun dismissNotification() {
        FlowPBXConnectionService.appContext?.let {
            IncomingCallNotificationHelper.dismiss(it)
        }
    }
}
