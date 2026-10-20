package dev.cxpalm

import android.os.SystemClock
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import kotlinx.coroutines.delay
import org.json.JSONObject
import java.util.Locale

internal data class TurnTiming(val status: String, val started: Long?, val duration: Long?)
internal fun turnTiming(turn: JSONObject): TurnTiming {
    fun millis(key: String): Long? = (turn.opt(key) as? Number)?.toDouble()?.takeIf { it.isFinite() && it >= 0 && it <= 8640000000000000.0 }?.toLong()
    val start = millis("turnStartedAtMs")
    val end = millis("turnCompletedAtMs")
    val duration = millis("durationMs") ?: millis("turnDurationMs") ?: if(start != null && end != null && end >= start) end-start else null
    return TurnTiming(turn.optString("status"), start, duration)
}
internal fun durationText(ms: Long, english: Boolean): String {
    val seconds = ms/1000
    val pieces = mutableListOf<String>()
    if(seconds >= 3600) pieces += if(english) "${seconds/3600}h" else "${seconds/3600} 小时"
    if(seconds >= 60) pieces += if(english) "${seconds/60%60}m" else "${seconds/60%60} 分"
    pieces += if(english) "${seconds%60}s" else "${seconds%60} 秒"
    return pieces.joinToString(" ")
}
internal fun runningClock(timing: TurnTiming?, now: Long, live: Boolean): String? {
    val start=timing?.started ?: return null
    if(!live || timing.status != "inProgress" || now < start) return null
    val seconds=(now-start)/1000
    return if(seconds >= 3600) String.format(Locale.ROOT,"%d:%02d:%02d",seconds/3600,seconds/60%60,seconds%60)
        else String.format(Locale.ROOT,"%d:%02d",seconds/60,seconds%60)
}
internal fun timingFooterIndices(messages: List<Message>): Set<Int> = messages.indices.groupBy { messages[it].turn }.values.mapNotNull { indices ->
    indices.lastOrNull { messages[it].role == "Codex" } ?: indices.lastOrNull { messages[it].role != "You" }
}.toSet()

@Composable internal fun CompletedTurnTime(timing: TurnTiming?) {
    if(timing?.status !in listOf("completed", "failed", "interrupted")) return
    val duration=timing?.duration ?: return
    Text(tr("用时 {0}", durationText(duration, AppLanguage.code == "en")), style=MaterialTheme.typography.labelSmall, color=MaterialTheme.colorScheme.onSurfaceVariant)
}

@Composable internal fun RunningTurnTime(timing: TurnTiming?, observedAt: Long?, live: Boolean) {
    val origin=remember(timing?.started,observedAt) { (observedAt ?: System.currentTimeMillis()) to SystemClock.elapsedRealtime() }
    var now by remember(origin) { mutableLongStateOf(origin.first) }
    LaunchedEffect(origin,live) {
        if(live && timing?.status == "inProgress") while(true) {
            now=origin.first + SystemClock.elapsedRealtime()-origin.second
            delay(1000)
        }
    }
    if (!live) return
    val clock=runningClock(timing,now,true)
    val label=clock?.let { tr("执行中 · {0}",it) } ?: tr("执行中 · 等待官方计时")
    Text(label,modifier=Modifier.semantics { contentDescription=tr("本轮执行用时") },style=MaterialTheme.typography.labelMedium,color=MaterialTheme.colorScheme.primary)
}
