package dev.cxpalm

import android.os.Handler
import okhttp3.*
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

data class ConversationNotification(val id: String, val kind: String, val thread: String, val title: String, val computerId: String, val computerName: String)
const val TASK_NOTIFICATION_ID = 2
fun taskNotificationTag(event: ConversationNotification): String = "task:${event.computerId}:${event.id}"
fun conversationNotification(data: JSONObject, computer: Computer): ConversationNotification? {
    val id = data.optString("id"); val kind = data.optString("kind"); val thread = data.optString("thread"); val title = data.optString("title")
    if (!id.matches(Regex("[a-f0-9]{64}")) || kind !in listOf("complete", "approval") || thread.isBlank() || thread.length > 128 || title.length > 640) return null
    return ConversationNotification(id, kind, thread, title, computer.id, computer.name)
}
fun notificationIDs(saved: String?): List<String> = try {
    val array = JSONArray(saved ?: "[]")
    (0 until array.length()).mapNotNull { array.optString(it).takeIf { id -> id.matches(Regex("[a-f0-9]{64}")) } }.takeLast(256)
} catch (_: Exception) { emptyList() }

// Read-only sockets stay independent from the foreground chat and its pending mutations.
class NotificationConnections(private val http: OkHttpClient, private val main: Handler, private val store: SecureStore,
    private val received: (ConversationNotification) -> Unit, private val status: (String, String?) -> Unit,
    private val resynchronized: (String) -> Unit = {}) {
    private val connections = mutableMapOf<String, Connection>()
    fun sync(computers: List<Computer>) {
        connections.keys.toList().filter { id -> computers.none { it.id == id } }.forEach { connections.remove(it)?.close() }
        computers.forEach { computer ->
            val previous = connections[computer.id]
            if (previous == null || previous.stopped) { previous?.close(); connections[computer.id] = Connection(computer).also { it.open() } }
        }
    }
    fun stop() { connections.values.forEach { it.close() }; connections.clear() }
    private inner class Connection(val computer: Computer) {
        var socket: WebSocket? = null
        var generation = 0
        var retry = 0
        var stopped = false
        var subscribeID = ""
        val key = "notifications:${computer.id}"
        var seen = try { notificationIDs(store.get(key)) } catch (_: Exception) { emptyList() }
        var cursor = try { store.get(key + ":cursor") ?: seen.lastOrNull().orEmpty() } catch (_: Exception) { seen.lastOrNull().orEmpty() }
        fun checkpoint(value: String) {
            if (value.length > 128) return
            cursor = value
            try { store.put(key + ":cursor", value) } catch (_: Exception) { /* In-memory resume remains available. */ }
        }
        fun open() {
            if (stopped) return
            val gen = ++generation
            status(computer.id, "connecting")
            val request = Request.Builder().url(computer.origin.replaceFirst("https://", "wss://") + "/api/v1/ws")
                .header("Origin", computer.origin).header("Authorization", "Bearer ${computer.token}").build()
            socket = http.newWebSocket(request, object : WebSocketListener() {
                override fun onMessage(webSocket: WebSocket, text: String) { main.post {
                    if (gen != generation || stopped) return@post
                    try {
                        val event = JSONObject(text)
                        when (event.optString("type")) {
                            "hello" -> {
                                val capabilities = event.getJSONObject("data").optJSONArray("capabilities")
                                if ((0 until (capabilities?.length() ?: 0)).none { capabilities!!.optString(it) == "notifications.subscribe" }) {
                                    stopped = true; status(computer.id, "unsupported"); webSocket.close(1000, "gateway update required"); return@post
                                }
                                retry = 0; subscribeID = UUID.randomUUID().toString()
                                webSocket.send(JSONObject().put("id", subscribeID).put("method", "notifications.subscribe").put("cursor", cursor).toString())
                            }
                            "result" -> if (event.optString("id") == subscribeID) {
                                val data = event.getJSONObject("data")
                                objects(data.optJSONArray("events")).forEach(::accept)
                                checkpoint(notificationResumeCursor(data, cursor))
                                if (data.optBoolean("resynced") || data.optBoolean("gap")) resynchronized(computer.id)
                                status(computer.id, if (data.optString("error").isNotBlank()) "unavailable" else null)
                            }
                            "notification" -> accept(event.getJSONObject("data"))
                            "notifications.status" -> status(computer.id, if (event.getJSONObject("data").optString("error").isBlank()) null else "unavailable")
                            "error" -> status(computer.id, "unavailable")
                        }
                    } catch (_: Exception) { status(computer.id, "invalid") }
                } }
                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) { main.post {
                    if (gen != generation || stopped) return@post
                    if (response?.code in listOf(401, 403)) { stopped = true; status(computer.id, "revoked"); return@post }
                    lost(gen)
                } }
                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) { main.post {
                    if (code == 1008) { if (gen == generation) { stopped = true; status(computer.id, "revoked") } } else lost(gen)
                } }
            })
        }
        fun accept(data: JSONObject) {
            val event = conversationNotification(data, computer) ?: return
            if (event.id in seen) return
            checkpoint(event.id)
            seen = (seen + event.id).takeLast(256)
            try { store.put(key, JSONArray(seen).toString()) } catch (_: Exception) { /* Keep in-memory deduplication; do not drop the alert. */ }
            received(event)
        }
        fun lost(gen: Int) {
            if (gen != generation || stopped) return
            generation++; socket = null; status(computer.id, "disconnected")
            val next = generation
            val wait = (1000L shl retry.coerceAtMost(5)).coerceAtMost(30000L); retry++
            main.postDelayed({ if (!stopped && next == generation) open() }, wait)
        }
        fun close() { stopped = true; generation++; socket?.close(1000, "monitor stopped"); socket = null }
    }
}

fun notificationResumeCursor(data: JSONObject, previous: String): String = when {
    data.has("cursor") && data.optString("cursor").length <= 128 -> data.optString("cursor")
    data.optBoolean("gap") -> ""
    else -> previous
}
