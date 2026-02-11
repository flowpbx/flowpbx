package com.flowpbx.mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.wifi.WifiManager
import android.os.IBinder
import androidx.core.app.NotificationCompat

/**
 * Foreground service that keeps the app alive during active calls.
 *
 * On Android, if the user navigates away from the app (or the system tries to
 * reclaim resources), a background app can be killed — dropping the SIP call.
 * This foreground service shows a persistent notification while a call is active,
 * which tells the OS to keep the process alive.
 *
 * Started when a call connects, stopped when the last call ends.
 */
class CallForegroundService : Service() {

    companion object {
        private const val CHANNEL_ID = "flowpbx_active_call"
        private const val NOTIFICATION_ID = 9002
        private const val EXTRA_CALLER = "caller"

        /** Start the foreground service for an active call. */
        fun start(context: Context, caller: String?) {
            val intent = Intent(context, CallForegroundService::class.java).apply {
                if (caller != null) putExtra(EXTRA_CALLER, caller)
            }
            context.startForegroundService(intent)
        }

        /** Stop the foreground service when the call ends. */
        fun stop(context: Context) {
            context.stopService(Intent(context, CallForegroundService::class.java))
        }
    }

    private var wifiLock: WifiManager.WifiLock? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        acquireWifiLock()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val caller = intent?.getStringExtra(EXTRA_CALLER) ?: "Unknown"
        val notification = buildNotification(caller)

        startForeground(
            NOTIFICATION_ID,
            notification,
            ServiceInfo.FOREGROUND_SERVICE_TYPE_PHONE_CALL
        )

        return START_NOT_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        releaseWifiLock()
        super.onDestroy()
    }

    /**
     * Acquire a WiFi lock to prevent the WiFi radio from going to sleep
     * during an active VoIP call. Without this, WiFi can be disabled by
     * the system when the screen turns off, dropping the call.
     */
    private fun acquireWifiLock() {
        if (wifiLock?.isHeld == true) return
        val wifiManager = applicationContext.getSystemService(Context.WIFI_SERVICE) as? WifiManager
            ?: return
        wifiLock = wifiManager.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "flowpbx:call")
        wifiLock?.acquire()
    }

    /** Release the WiFi lock when the call ends. */
    private fun releaseWifiLock() {
        wifiLock?.let {
            if (it.isHeld) it.release()
        }
        wifiLock = null
    }

    private fun createNotificationChannel() {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (nm.getNotificationChannel(CHANNEL_ID) != null) return

        val channel = NotificationChannel(
            CHANNEL_ID,
            "Active Call",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Shown while a call is in progress to keep the app alive"
            setShowBadge(false)
            lockscreenVisibility = Notification.VISIBILITY_PUBLIC
        }
        nm.createNotificationChannel(channel)
    }

    private fun buildNotification(caller: String): Notification {
        // Tap notification to return to the in-call screen.
        val tapIntent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
            action = "com.flowpbx.mobile.RETURN_TO_CALL"
        }
        val tapPending = PendingIntent.getActivity(
            this, NOTIFICATION_ID, tapIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        // Hang up action.
        val hangupIntent = Intent(this, CallActionReceiver::class.java).apply {
            action = CallActionReceiver.ACTION_REJECT
            // Use first active connection UUID so the receiver can route it.
            val uuid = FlowPBXConnectionService.connections.keys.firstOrNull() ?: ""
            putExtra("uuid", uuid)
        }
        val hangupPending = PendingIntent.getBroadcast(
            this, NOTIFICATION_ID + 1, hangupIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.sym_action_call)
            .setContentTitle("Call in progress")
            .setContentText(caller)
            .setOngoing(true)
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setContentIntent(tapPending)
            .addAction(
                NotificationCompat.Action.Builder(
                    android.R.drawable.ic_menu_close_clear_cancel,
                    "Hang Up",
                    hangupPending
                ).build()
            )
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .build()
    }
}
