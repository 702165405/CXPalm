package dev.cxpalm

import android.util.AtomicFile
import java.io.File
import java.security.MessageDigest
import java.util.concurrent.Executors
import org.json.JSONObject
import org.json.JSONArray

internal fun readingScope(computer: Computer): String = MessageDigest.getInstance("SHA-256")
    .digest("${computer.id}\u0000${computer.origin}\u0000${computer.token}".toByteArray()).joinToString("") { "%02x".format(it) }
internal fun offlineSnapshot(snapshot: JSONObject?): JSONObject? = snapshot?.let {
    JSONObject(it.toString()).apply {
        remove("requests");remove("hasUnreadTurn");remove("readToken")
        put("threadRuntimeStatus",JSONObject().put("type","unknown"))
    }
}
internal fun offlineSessions(rows: List<JSONObject>): List<JSONObject> = rows.map {
    JSONObject(it.toString()).put("live",false).put("status","unknown").put("unread",JSONObject.NULL).apply { remove("readToken") }
}
internal fun readingPayload(state: ViewState): JSONObject = JSONObject().put("snapshot",offlineSnapshot(state.snapshot))
    .put("historyPage",state.historyPage).put("historyIndex",state.historyIndex).put("archived",state.threadArchived)
    .put("catalog",state.catalog?.let { JSONObject().put("models",it.optJSONArray("models")) })
internal fun restoreReading(state: ViewState, payload: JSONObject?): ViewState = if(payload==null) state else state.copy(
    snapshot=offlineSnapshot(payload.optJSONObject("snapshot")),historyPage=payload.optJSONObject("historyPage"),historyIndex=payload.optJSONObject("historyIndex"),
    catalog=payload.optJSONObject("catalog"),threadArchived=payload.optBoolean("archived"),cached=true,readToken="",desktopConnected=false,sequence=-1)

internal class ReadingMemory {
    private val chats=linkedMapOf<String,ViewState>()
    private val lists=linkedMapOf<String,ViewState>()
    @Synchronized fun remember(state: ViewState) {
        val computer=state.current ?: return
        val scope=readingScope(computer)
        if(state.sessions.isNotEmpty() && state.connected) { lists.remove(scope);lists[scope]=state }
        if(state.thread!=null && state.snapshot!=null && !state.cached) {
            val key="$scope:${state.thread}";chats.remove(key);chats[key]=state
        }
        while(chats.size>12) chats.remove(chats.keys.first())
        while(lists.size>4) lists.remove(lists.keys.first())
    }
    @Synchronized fun get(state: ViewState): ViewState {
        val scope=state.current?.let(::readingScope) ?: return state
        val list=lists[scope]
        val row=chats["$scope:${state.thread}"]
        val restored=if(row!=null) restoreReading(state,readingPayload(row)) else state
        return if(list!=null && (!state.connected || state.sessions.isEmpty())) restored.copy(sessions=offlineSessions(list.sessions),projects=list.projects,archived=list.archived,project=list.project,sessionSearch=list.sessionSearch) else restored
    }
    @Synchronized fun forget(scope: String) { lists.remove(scope);chats.keys.filter { it.startsWith("$scope:") }.forEach(chats::remove) }
    @Synchronized fun records(scope: String): Pair<ViewState?,List<ViewState>> = lists[scope] to chats.filterKeys { it.startsWith("$scope:") }.values.toList()
    @Synchronized fun import(scope: String, value: JSONObject, current: ViewState) {
        if(value.optInt("schema")!=1 || value.optString("scope")!=scope || System.currentTimeMillis()-value.optLong("at") !in 0..604800000L)return
        val restored=current.copy(sessions=offlineSessions(objects(value.optJSONArray("sessions"))),projects=objects(value.optJSONArray("projects")),archived=value.optBoolean("archived"),project=value.optString("project"),sessionSearch=value.optString("search"))
        if(!lists.containsKey(scope)) lists[scope]=restored
        objects(value.optJSONArray("chats")).takeLast(12).forEach { row ->
            val thread=row.optString("thread")
            if(thread.isNotBlank() && thread.length<=128 && !chats.containsKey("$scope:$thread")) chats["$scope:$thread"]=restoreReading(restored.copy(thread=thread),row).copy(cached=false)
        }
        while(chats.size>12) chats.remove(chats.keys.first())
        while(lists.size>4) lists.remove(lists.keys.first())
    }
}
internal class ReadingCache(private val folder: File, private val secure: SecureStore) {
    val memory=ReadingMemory()
    private val io=Executors.newSingleThreadExecutor { task -> Thread(task,"cxpalm-reading-cache").apply { isDaemon=true } }
    fun load(state: ViewState, complete: () -> Unit) {
        val scope=state.current?.let(::readingScope) ?: return
        io.execute { runCatching {
            val file=File(folder,"$scope.json")
            if(file.length() in 1..6000000L) memory.import(scope,JSONObject(secure.unseal(scope,file.readText())),state)
        };complete() }
    }
    fun clear(computer: Computer) {
        val scope=readingScope(computer);memory.forget(scope)
        io.execute { memory.forget(scope);runCatching { File(folder,"$scope.json").delete() } }
    }
    fun save(state: ViewState) {
        val scope=state.current?.let(::readingScope) ?: return
        io.execute { runCatching {
            val (list,chats)=memory.records(scope)
            val value=JSONObject().put("schema",1).put("scope",scope).put("at",System.currentTimeMillis())
                .put("sessions",JSONArray(offlineSessions(list?.sessions.orEmpty()))).put("projects",JSONArray(list?.projects.orEmpty()))
                .put("archived",list?.archived ?: false).put("project",list?.project.orEmpty()).put("search",list?.sessionSearch.orEmpty())
                .put("chats",JSONArray(chats.map { readingPayload(it).put("thread",it.thread) }))
            val raw=value.toString()
            if(raw.toByteArray().size>4000000)return@runCatching
            folder.mkdirs();val atomic=AtomicFile(File(folder,"$scope.json"));val stream=atomic.startWrite()
            try { stream.write(secure.seal(scope,raw).toByteArray());atomic.finishWrite(stream) } catch(error:Exception) { atomic.failWrite(stream);throw error }
            // Only encrypted cache-owned files are evicted; drafts and credentials are elsewhere.
            val files=folder.listFiles()?.filter { it.name.matches(Regex("[a-f0-9]{64}\\.json")) && it.canonicalFile.parentFile==folder.canonicalFile }?.sortedByDescending { it.lastModified() }.orEmpty()
            var used=0L
            files.forEachIndexed { index,file -> used+=file.length();if(index>=4 || used>16000000L) file.delete() }
        } }
    }
}
