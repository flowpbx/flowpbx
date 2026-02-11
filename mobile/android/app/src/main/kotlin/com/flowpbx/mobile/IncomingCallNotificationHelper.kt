package com.flowpbx.mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.media.AudioAttributes
import android.media.RingtoneManager
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

/**
 * Helper to show a heads-up notification for incoming calls on Android.
 *
 * Self-managed ConnectionService calls do NOT get the system incoming-call UI,
 * so we display a high-priority notification with Answer and Reject actions.
 * This notification appears as a heads-up banner even when the app is in the
 * background, which is the standard Android pattern for VoIP apps.
 */
object IncomingCallNotificationHelper {

    private const val CHANNEL_ID = "flowpbx_incoming_call"
    private const val NOTIFICATION_ID = 9001

    /** Ensure the notification channel exists. Safe to call multiple times. */
    fun createChannel(context: Context) {
        val nm = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (nm.getNotificationChannel(CHANNEL_ID) != null) return

        val ringtoneUri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_RINGTONE)
        val audioAttrs = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_NOTIFICATION_RINGTONE)
            .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
            .build()

        val channel = NotificationChannel(
            CHANNEL_ID,
            "Incoming Calls",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "Incoming call notifications with answer and reject actions"
            setSound(ringtoneUri, audioAttrs)
            enableVibration(true)
            vibrationPattern = longArrayOf(0, 1000, 500, 1000)
            lockscreenVisibility = Notification.VISIBILITY_PUBLIC
            setBypassDnd(true)
        }
        nm.createNotificationChannel(channel)
    }

    /**
     * Show a heads-up incoming call notification.
     *
     * @param context Application or service context.
     * @param uuid    The unique call identifier (used to route Answer/Reject).
     * @param handle  The caller number or SIP URI.
     * @param displayName Optional display name for the caller.
     */
    fun show(context: Context, uuid: String, handle: String, displayName: String?) {
        createChannel(context)

        val callerLabel = if (!displayName.isNullOrEmpty()) "$displayName ($handle)" else handle

        // Full-screen intent — opens the app's incoming call screen.
        val fullScreenIntent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP
            action = "com.flowpbx.mobile.INCOMING_CALL"
            putExtra("uuid", uuid)
        }
        val fullScreenPending = PendingIntent.getActivity(
            context, NOTIFICATION_ID, fullScreenIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        // Answer action.
        val answerIntent = Intent(context, CallActionReceiver::class.java).apply {
            action = CallActionReceiver.ACTION_ANSWER
            putExtra("uuid", uuid)
        }
        val answerPending = PendingIntent.getBroadcast(
            context, NOTIFICATION_ID + 1, answerIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        // Reject action.
        val rejectIntent = Intent(context, CallActionReceiver::class.java).apply {
            action = CallActionReceiver.ACTION_REJECT
            putExtra("uuid", uuid)
        }
        val rejectPending = PendingIntent.getBroadcast(
            context, NOTIFICATION_ID + 2, rejectIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.sym_call_incoming)
            .setContentTitle("Incoming Call")
            .setContentText(callerLabel)
            .setPriority(NotificationCompat.PRIORITY_MAX)
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setOngoing(true)
            .setAutoCancel(false)
            .setFullScreenIntent(fullScreenPending, true)
            .addAction(
                NotificationCompat.Action.Builder(
                    android.R.drawable.sym_action_call,
                    "Answer",
                    answerPending
                ).build()
            )
            .addAction(
                NotificationCompat.Action.Builder(
                    android.R.drawable.sym_call_missed,
                    "Reject",
                    rejectPending
                ).build()
            )
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .build()

        try {
            NotificationManagerCompat.from(context).notify(NOTIFICATION_ID, notification)
        } catch (_: SecurityException) {
            // POST_NOTIFICATIONS permission not granted (Android 13+). Non-fatal —
            // ConnectionService still functions; the user just won't see the banner.
        }
    }

    /** Dismiss the incoming call notification. */
    fun dismiss(context: Context) {
        NotificationManagerCompat.from(context).cancel(NOTIFICATION_ID)
    }
}
