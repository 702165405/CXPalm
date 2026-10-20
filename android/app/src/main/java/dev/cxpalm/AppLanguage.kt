package dev.cxpalm

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import org.json.JSONObject
import java.util.Locale

object AppLanguage {
    var code by mutableStateOf("zh-CN")
        private set
    private var loaded = false
    private var catalog = LanguageCatalog(emptyMap())
    val locale: Locale get() = Locale.forLanguageTag(code)
    @Synchronized fun initialize(context: Context) {
        if (loaded) return
        val app = context.applicationContext
        val json = JSONObject(app.assets.open("en.json").bufferedReader(Charsets.UTF_8).use { it.readText() })
        catalog = LanguageCatalog(json.keys().asSequence().associateWith { json.getString(it) })
        code = languageCode(app.getSharedPreferences("cxpalm-ui", Context.MODE_PRIVATE).getString("language", null), Locale.getDefault().language)
        loaded = true
    }
    fun select(context: Context, value: String) {
        require(value == "zh-CN" || value == "en")
        if (!context.applicationContext.getSharedPreferences("cxpalm-ui", Context.MODE_PRIVATE).edit().putString("language", value).commit()) {
            error(tr("无法保存语言，请稍后重试"))
        }
        code = value
        if (Runtime.backgroundActive && notificationsAllowed(context)) {
            try { context.getSystemService(android.app.NotificationManager::class.java).notify(1, connectionNotification(context)) }
            catch (_: SecurityException) { }
        }
    }
    fun text(source: String, vararg values: Any?): String = catalog.text(source, code, *values)
}

fun tr(source: String, vararg values: Any?): String = AppLanguage.text(source, *values)
