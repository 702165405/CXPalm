package dev.cxpalm

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

object AppAppearance {
    var mode by mutableStateOf("system")
        private set
    fun initialize(context: Context) {
        mode = appearanceMode(context.applicationContext.getSharedPreferences("cxpalm-ui", Context.MODE_PRIVATE).getString("appearance", null))
    }
    fun select(context: Context, value: String) {
        require(value in listOf("system", "light", "dark"))
        check(context.applicationContext.getSharedPreferences("cxpalm-ui", Context.MODE_PRIVATE).edit().putString("appearance", value).commit()) {
            tr("无法保存外观，请稍后重试")
        }
        mode = value
    }
}
