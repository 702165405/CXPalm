package dev.cxpalm

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.IBinder
import android.net.Uri
import io.github.u702165405.cxpalm.R

fun notificationsAllowed(context: Context): Boolean = context.getSystemService(NotificationManager::class.java).areNotificationsEnabled() &&
    (Build.VERSION.SDK_INT < 33 || context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED)

fun localNotification(context: Context, channel: String, id: Int, title: String, text: String, updates: Boolean = false, computer: String? = null, thread: String? = null, raw: Boolean = false, tag: String? = null) {
    AppLanguage.initialize(context)
    if (!notificationsAllowed(context)) return
    val manager = context.getSystemService(NotificationManager::class.java)
    manager.createNotificationChannel(NotificationChannel(channel, if (updates) tr("版本更新") else tr("Codex 会话提醒"), NotificationManager.IMPORTANCE_DEFAULT))
    val intent = Intent(context, MainActivity::class.java).putExtra("updates", updates).putExtra("notificationComputer", computer).putExtra("notificationThread", thread)
        .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP)
    // PendingIntent identity excludes extras; qualify task clicks by computer and event.
    if (tag != null) intent.data = Uri.Builder().scheme("cxpalm").authority("notification").appendPath(tag).build()
    val open = PendingIntent.getActivity(context, id, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    try { manager.notify(tag, id, Notification.Builder(context, channel).setSmallIcon(R.drawable.ic_notification)
        .setContentTitle(if (raw) title else tr(title)).setContentText(if (raw) text else tr(text)).setContentIntent(open).setAutoCancel(tag == null).build()) }
    catch (_: SecurityException) { }
}

fun connectionNotification(context: Context): Notification {
    val manager = context.getSystemService(NotificationManager::class.java)
    manager.createNotificationChannel(NotificationChannel("connection", tr("电脑连接"), NotificationManager.IMPORTANCE_LOW))
    val open = PendingIntent.getActivity(context, 1, Intent(context, MainActivity::class.java), PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    return Notification.Builder(context, "connection").setSmallIcon(R.drawable.ic_notification)
        .setContentTitle(tr("CXPalm 后台连接已开启")).setContentText(tr("仅本机通知；可在应用设置中停止。后台连接为尽力保持。"))
        .setContentIntent(open).setOngoing(true).build()
}

class ConnectionService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
    override fun onCreate() {
        super.onCreate()
        AppLanguage.initialize(this)
        startForeground(1, connectionNotification(this))
        Runtime.backgroundActive = true
        Runtime.client(this).startMonitoring()
    }
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = START_NOT_STICKY
    override fun onDestroy() { Runtime.backgroundActive = false; if (!Runtime.visible) Runtime.client(this).stopMonitoring(); super.onDestroy() }
}
