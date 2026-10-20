package dev.cxpalm

import android.Manifest
import android.app.NotificationManager
import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.FileProvider
import androidx.core.net.toUri
import androidx.lifecycle.compose.collectAsStateWithLifecycle

object Runtime {
    private var instance: BridgeClient? = null
    @Suppress("StaticFieldLeak") // UpdateManager retains only applicationContext, never an Activity.
    private var updater: UpdateManager? = null
    var visible = false
    var backgroundActive by mutableStateOf(false)
    @Synchronized fun client(context: Context): BridgeClient = instance ?: BridgeClient(context).also { instance = it }
    @Synchronized fun updates(context: Context): UpdateManager = updater ?: UpdateManager(context).also { updater = it }
}

class MainActivity : ComponentActivity() {
    private var notificationAllowed by mutableStateOf(false)
    private var showUpdates by mutableStateOf(false)
    private var awaitingInstallPermission = false
    private var installing = false
    private val notifications = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        notificationAllowed = granted
        if (granted) startForegroundService(Intent(this, ConnectionService::class.java))
    }
    private val foregroundNotifications = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        notificationAllowed = granted && notificationsAllowed(this)
    }
    private val unknownSources = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
        awaitingInstallPermission = false
        if (packageManager.canRequestPackageInstalls()) installUpdate()
        else Runtime.updates(this).installError("未允许安装；下载仍保留，可稍后再次点击安装")
    }
    private val installer = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
        installing = false
        Runtime.updates(this).installError("若已取消系统安装，可继续使用或再次点击安装")
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        AppLanguage.initialize(this)
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        awaitingInstallPermission = savedInstanceState?.getBoolean("install-permission") ?: false
        showUpdates = intent.getBooleanExtra("updates", false)
        val client = Runtime.client(this)
        openNotification(intent)
        val updates = Runtime.updates(this)
        setContent {
            CxpalmTheme {
                val state by client.state.collectAsStateWithLifecycle()
                val update by updates.state.collectAsStateWithLifecycle()
                PalmApp(state, client, update, updates, notificationAllowed, ::enableNotifications, ::enableBackground,
                    ::stopBackground, ::notificationSettings,
                    ::installUpdate, showUpdates, { showUpdates = false })
            }
        }
    }

    override fun onNewIntent(intent: Intent) { super.onNewIntent(intent); setIntent(intent); openNotification(intent); if (intent.getBooleanExtra("updates", false)) showUpdates = true }
    override fun onSaveInstanceState(outState: Bundle) { outState.putBoolean("install-permission", awaitingInstallPermission); super.onSaveInstanceState(outState) }
    override fun onStart() { super.onStart(); Runtime.visible = true; Runtime.client(this).startMonitoring() }
    override fun onResume() {
        super.onResume()
        notificationAllowed = notificationsAllowed(this)
        Runtime.updates(this).foreground()
    }
    override fun onStop() { Runtime.visible = false; if (!Runtime.backgroundActive) Runtime.client(this).stopMonitoring(); super.onStop() }

    private fun openNotification(intent: Intent) {
        val computer = intent.getStringExtra("notificationComputer") ?: return
        val thread = intent.getStringExtra("notificationThread") ?: return
        intent.removeExtra("notificationComputer")
        intent.removeExtra("notificationThread")
        if (Runtime.client(this).openNotification(computer, thread)) {
            val tag = intent.data?.takeIf { it.scheme == "cxpalm" && it.authority == "notification" }?.lastPathSegment
            if (tag?.matches(Regex("task:${Regex.escape(computer)}:[a-f0-9]{64}")) == true)
                getSystemService(NotificationManager::class.java).cancel(tag, TASK_NOTIFICATION_ID)
        }
    }
    private fun enableNotifications() {
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED)
            foregroundNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        else notificationSettings()
    }
    private fun enableBackground() {
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED)
            notifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        else startForegroundService(Intent(this, ConnectionService::class.java))
    }
    private fun notificationSettings() {
        startActivity(Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, packageName))
    }
    private fun stopBackground() { stopService(Intent(this, ConnectionService::class.java)) }
    private fun installUpdate() {
        if (installing || awaitingInstallPermission) return
        val updates = Runtime.updates(this)
        if (updates.state.value.phase != UpdatePhase.READY) return
        if (!packageManager.canRequestPackageInstalls()) {
            try {
                awaitingInstallPermission = true
                unknownSources.launch(Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, "package:$packageName".toUri()))
            } catch (_: Exception) { awaitingInstallPermission = false; updates.installError("无法打开安装授权页，请在系统设置中允许此应用安装更新") }
            return
        }
        installing = true
        updates.forInstall { file ->
            if (file == null) { installing = false; return@forInstall }
            try {
                val uri = FileProvider.getUriForFile(this, "$packageName.updates", file)
                installer.launch(Intent(Intent.ACTION_VIEW).setDataAndType(uri, "application/vnd.android.package-archive")
                    .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION).apply { clipData = ClipData.newRawUri("CXPalm update", uri) })
            } catch (_: Exception) { installing = false; updates.installError("系统未能打开安装器，请稍后重试") }
        }
    }
}
