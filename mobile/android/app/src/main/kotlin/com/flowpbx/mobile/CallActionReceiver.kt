package com.flowpbx.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/**
 * Receives Answer/Reject actions from the incoming call heads-up notification.
 *
 * Routes the action to the corresponding FlowPBXConnection so the Telecom
 * framework and Flutter layer both see the state change.
 */
class CallActionReceiver : BroadcastReceiver() {

    companion object {
        const val ACTION_ANSWER = "com.flowpbx.mobile.ACTION_ANSWER_CALL"
        const val ACTION_REJECT = "com.flowpbx.mobile.ACTION_REJECT_CALL"
    }

    override fun onReceive(context: Context, intent: Intent) {
        val uuid = intent.getStringExtra("uuid") ?: return
        val connection = FlowPBXConnectionService.connections[uuid] ?: return

        IncomingCallNotificationHelper.dismiss(context)

        when (intent.action) {
            ACTION_ANSWER -> {
                // Delegate to Connection.onAnswer() which sets state and notifies Flutter.
                connection.onAnswer()
                // Bring the app to the foreground for the in-call screen.
                val launchIntent = Intent(context, MainActivity::class.java).apply {
                    flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP
                    action = "com.flowpbx.mobile.CALL_ANSWERED"
                    putExtra("uuid", uuid)
                }
                context.startActivity(launchIntent)
            }
            ACTION_REJECT -> {
                // Delegate to Connection.onReject() which disconnects and notifies Flutter.
                connection.onReject()
            }
        }
    }
}
