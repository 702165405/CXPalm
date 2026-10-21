package dev.cxpalm

import android.content.Context
import android.os.Handler
import android.os.Looper
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.util.UUID
import java.util.concurrent.TimeUnit

data class Computer(val id: String, val name: String, val origin: String, val token: String, val remember: Boolean = true, val expires: Long = Long.MAX_VALUE) {
    override fun toString() = "Computer(id=$id, origin=$origin, token=<redacted>)"
}
data class ViewState(val operationFailures: Map<String, JSONObject> = emptyMap(), val pendingOperations: Map<String, JSONObject> = emptyMap(), val sessionSync: JSONObject? = null, val uploadBusy: Boolean = false, val goalDraft: Boolean = false, val cached: Boolean = false, val readToken: String = "", val sharedReadSync: Boolean = false, val computers: List<Computer> = emptyList(), val current: Computer? = null, val status: String = "Disconnected", val sessions: List<JSONObject> = emptyList(), val projects: List<JSONObject> = emptyList(), val archived: Boolean = false, val project: String = "", val sessionSearch: String = "", val sessionLimit: Int = 100, val hasMore: Boolean = false, val threadArchived: Boolean = false, val thread: String? = null, val snapshot: JSONObject? = null, val historyPage: JSONObject? = null, val historyIndex: JSONObject? = null, val historyLoading: Boolean = false, val historyTarget: String? = null, val connected: Boolean = false, val error: String? = null, val pending: String? = null, val desktopConnected: Boolean = false, val catalog: JSONObject? = null, val skills: Set<String> = emptySet(), val draftRevision: Long = 0, val sendScrollRevision: Long = 0, val sendUserBefore: String? = null, val sequence: Long = -1, val notificationStatus: Map<String, String?> = emptyMap(), val notificationNavigation: Long = 0)

class BridgeClient(context: Context) {
    private val store = SecureStore(context.applicationContext)
    private val main = Handler(Looper.getMainLooper())
    private val reading=ReadingCache(java.io.File(context.noBackupFilesDir,"reading-v1"),store)
    private var saveReading: Runnable? = null
    private var cacheRestoreTicket=0
    private val client = OkHttpClient.Builder().connectTimeout(10, TimeUnit.SECONDS).readTimeout(0, TimeUnit.SECONDS)
        .followRedirects(false).followSslRedirects(false).retryOnConnectionFailure(false).build()
    private val mutable = MutableStateFlow(ViewState())
    val state = mutable.asStateFlow()
    private var socket: WebSocket? = null
    private var stream=SnapshotStream()
    private val bodyCache=linkedMapOf<String,Message>()
    @Volatile private var generation = 0
    private var retry = 0
    private var wanted = false
    private var subscription = 0
    private var listSubscription = ""
    private var historyCall: String? = null
    private var historyAttempt = ""
    private var readAttempt = ""
    private val calls = mutableMapOf<String, (JSONObject) -> Unit>()
    private val receiptQueries = mutableMapOf<String, String>()
    var loginBusy by androidx.compose.runtime.mutableStateOf(false)
        private set
    private val app = context.applicationContext
    private val monitors = NotificationConnections(client, main, store, { event ->
        localNotification(app, "tasks", TASK_NOTIFICATION_ID,
            "${event.computerName} · ${if (event.kind == "complete") tr("Codex 任务已完成") else tr("Codex 有新的待审批请求")}", event.title,
            computer = event.computerId, thread = event.thread, raw = true, tag = taskNotificationTag(event))
    }, { computer, status -> update { it.copy(notificationStatus = it.notificationStatus + (computer to status)) } },
        { computer -> if (mutable.value.current?.id == computer && mutable.value.connected) refresh() })
    fun startMonitoring() { monitors.sync(mutable.value.computers) }
    fun stopMonitoring() { monitors.stop(); update { it.copy(notificationStatus = emptyMap()) } }
    fun openNotification(computer: String, thread: String): Boolean {
        if (thread.isBlank() || thread.length > 128) return false
        val target = mutable.value.computers.firstOrNull { it.id == computer } ?: return false
        store.put("thread:${target.id}", thread)
        if (mutable.value.current?.id == target.id && mutable.value.connected) subscribe(thread)
        else { connect(target); update { it.copy(thread = thread, snapshot = null, desktopConnected = false) } }
        update { it.copy(notificationNavigation = it.notificationNavigation + 1) }
        return true
    }
    init {
        try { val saved = JSONArray(store.get("computers") ?: "[]"); mutable.value = mutable.value.copy(computers = objects(saved).map { Computer(it.getString("id"), it.getString("name"), normalizeOrigin(it.getString("origin")), it.getString("token"),it.optBoolean("remember",true),it.optLong("expires",Long.MAX_VALUE)) }) }
        catch (_: Exception) { mutable.value = mutable.value.copy(error = "加密的本地登录数据无法读取，请重新登录") }
    }
    private val restore = Runnable {
        val saved = store.get("current")
        mutable.value.computers.firstOrNull { it.id == saved }?.let(::connect)
    }
    init { main.post { restore.run(); startMonitoring() } }
    private fun update(fn: (ViewState) -> ViewState) {
        val old=mutable.value;val changed=fn(old);val next=changed.copy(pending=changed.pendingFor(changed.thread)?.optString("id"));mutable.value=next
        if(next.snapshot!==old.snapshot || next.historyPage!==old.historyPage || next.historyIndex!==old.historyIndex || next.sessions!==old.sessions) {
            reading.memory.remember(next)
            saveReading?.let(main::removeCallbacks)
            saveReading=Runnable { reading.save(next) }.also { main.postDelayed(it,750) }
        }
    }
    private fun restoreCached() {
        val restoreTicket=++cacheRestoreTicket
        val ticket=mutable.value;val scope=ticket.current?.let(::readingScope) ?: return
        update { reading.memory.get(it) }
        reading.load(ticket) { main.post {
            val current=mutable.value
            if(restoreTicket==cacheRestoreTicket && current.current?.let(::readingScope)==scope && current.thread==ticket.thread && (current.snapshot==null || current.cached)) update { reading.memory.get(it) }
        } }
    }
    fun clearError() = update { it.copy(error = null) }
    fun reportError(message: String) = update { it.copy(error = message) }
    fun login(name: String, address: String, username: String, password: String, remember: Boolean, done: () -> Unit) {
        if(loginBusy) return
        try {
            val origin=loginOrigin(address)
            require(username.isNotBlank() && username.toByteArray(Charsets.UTF_8).size<=80 && password.isNotEmpty() && password.toByteArray(Charsets.UTF_8).size<=256) { "请输入有效的账号和密码。" }
            val previous=mutable.value.computers.filter { it.origin==origin }
            if(previous.any { hasPending(it) }) { reportError("操作结果未知，请先核对当前电脑，再切换连接。");return }
            loginBusy=true
            val request=Request.Builder().url("$origin/api/v1/login").header("Origin",origin)
                .post(JSONObject().put("username",username.trim()).put("password",password).put("remember",remember).put("name","Android App").toString().toRequestBody("application/json".toMediaType())).build()
            val call=client.newCall(request);call.timeout().timeout(20,TimeUnit.SECONDS)
            call.enqueue(object: Callback {
                override fun onFailure(call: Call,e: IOException) { main.post { loginBusy=false;reportError("登录连接未确认，请检查网络后重新登录。") } }
                override fun onResponse(call: Call,response: Response) {
                    val code=response.code
                    val body=try { response.use { it.peekBody(16385).string() } } catch(_: Exception) { main.post { loginBusy=false;reportError("无法读取登录结果，请重试。") };return }
                    main.post {
                        try {
                            if(code!=200){reportError(loginHTTPError(code));return@post}
                            val computer=loginComputer(body,origin,name,remember)
                            val list=mutable.value.computers.filterNot { it.origin==origin }+computer
                            store.put("computers",savedComputers(list));previous.forEach(reading::clear)
                            update { it.copy(computers=list,error=null) };connect(computer);done()
                        }catch(_: Exception){reportError("无法验证或保存登录结果，请重试。")}
                        finally{loginBusy=false}
                    }
                }
            })
        }catch(e: Exception){loginBusy=false;reportError(e.message ?: "请输入有效的账号和密码。")}
    }
    fun logout(computer: Computer) {
        if(loginBusy)return
        if(hasPending(computer)){reportError("操作结果未知，请先核对当前电脑，再切换连接。");return}
        loginBusy=true
        val request=Request.Builder().url("${computer.origin}/api/v1/logout").header("Origin",computer.origin).header("Authorization","Bearer ${computer.token}")
            .post("{}".toRequestBody("application/json".toMediaType())).build()
        val call=client.newCall(request);call.timeout().timeout(20,TimeUnit.SECONDS)
        call.enqueue(object: Callback {
            override fun onFailure(call: Call,e: IOException){main.post {loginBusy=false;reportError("退出结果未确认，请联网后重试，或在电脑撤销设备授权。")}}
            override fun onResponse(call: Call,response: Response){val code=response.use {it.code};main.post {
                try {
                    if(code!=200){reportError(loginHTTPError(code));return@post}
                    val list=mutable.value.computers.filterNot {it.id==computer.id}
                    store.put("computers",savedComputers(list))
                    if(mutable.value.current?.id==computer.id){disconnectChat();store.put("current",null);update {ViewState(computers=list)}}else update {it.copy(computers=list)}
                    reading.clear(computer);startMonitoring()
                }catch(_: Exception){reportError("无法保存退出结果，请重试。") }finally{loginBusy=false}
            }}
        })
    }
    fun connect(computer: Computer) {
        if(computer.expires<=System.currentTimeMillis()/1000){reportError("登录已过期，请使用账号密码重新登录。");return}
        if(computer.id!=mutable.value.current?.id) goalMode(false)
        val current = mutable.value
        val same = current.current?.id == computer.id
        disconnectChat(); wanted = true; store.put("current", computer.id)
        val restoredThread = if (same) current.thread else store.get("thread:${computer.id}")
        update { computerConnectionState(it, computer, restoredThread, null, savedSkills(computer.id, restoredThread)).copy(pendingOperations=loadPending(computer)) }
        restoreCached();open(); startMonitoring()
    }

    fun disconnect() { stopMonitoring(); disconnectChat() }
    private fun disconnectChat() { wanted = false; generation++; socket?.close(1000, "disconnect"); socket = null; calls.clear(); receiptQueries.clear(); update { it.copy(connected = false, desktopConnected = false, status = "Disconnected",cached=it.snapshot!=null,readToken="",sharedReadSync=false,sessions=offlineSessions(it.sessions)) } }
    private fun open() {
        val computer = mutable.value.current ?: return; val gen = ++generation
        val socketStream=SnapshotStream();stream=socketStream
        val recoveryQueued=java.util.concurrent.atomic.AtomicBoolean(false)
        val lastFrame=java.util.concurrent.atomic.AtomicLong(android.os.SystemClock.elapsedRealtime())
        val watchdog=object:Runnable {
            override fun run(){
                if(gen!=generation)return
                if(android.os.SystemClock.elapsedRealtime()-lastFrame.get()>=30000){
                    val stale=socket;lost(gen,"Connection heartbeat expired");stale?.cancel()
                }else main.postDelayed(this,5000)
            }
        }
        main.postDelayed(watchdog,5000)
        update { it.copy(sequence = -1) }
        update { it.copy(status = "Connecting...", connected = false) }
        val request = Request.Builder().url(computer.origin.replaceFirst("https://", "wss://") + "/api/v1/ws").header("Origin", computer.origin).header("Authorization", "Bearer ${computer.token}").build()
        // Frames are parsed on the socket thread; bursts of chat.state collapse to the newest one before reaching the UI.
        val latestChatState = java.util.concurrent.atomic.AtomicReference<JSONObject?>(null)
        socket = client.newWebSocket(request, object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                if(gen!=generation)return
                val parsed = try { JSONObject(text) } catch (_: Exception) {
                    main.post { if (gen == generation) update { it.copy(error = "服务端响应不受支持，请更新或重新连接") } }; return
                }
                lastFrame.set(android.os.SystemClock.elapsedRealtime())
                val event=try {socketStream.accept(parsed) ?: return} catch(_: Exception) {
                    val selected=mutable.value.thread
                    if(recoveryQueued.compareAndSet(false,true))main.post {
                        if(gen==generation && mutable.value.thread==selected) selected?.let { command("chat.subscribe",JSONObject().put("thread",it).put("stream",true)) }
                        recoveryQueued.set(false)
                    }
                    return
                }
                if (event.optString("type") != "chat.state") { main.post { if (gen == generation) receive(event) }; return }
                event.optJSONObject("data")?.optJSONObject("snapshot")?.let(::conversationMessages)
                if (latestChatState.getAndSet(event) == null) main.post {
                    val latest = latestChatState.getAndSet(null)
                    if (latest != null && gen == generation) receive(latest)
                }
            }
            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) { main.post { if (gen == generation && response?.code in listOf(401, 403)) {
                wanted=false;++cacheRestoreTicket;saveReading?.let(main::removeCallbacks);reading.clear(computer)
                update { it.copy(snapshot=null,historyPage=null,historyIndex=null,sessions=emptyList(),projects=emptyList(),cached=false,readToken="") }
            }; lost(gen, "Connection lost${response?.code?.let { ": $it" } ?: ""}") } }
            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) { webSocket.close(code,reason) }
            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) { main.post { lost(gen, "Disconnected") } }
        })
    }
    private fun lost(gen: Int, message: String) {
        if (gen != generation) return
        val retryGeneration=++generation
        socket = null; calls.clear(); receiptQueries.clear(); update { it.copy(connected = false, desktopConnected = false, historyLoading = false, status = message,cached=it.snapshot!=null,readToken="",sharedReadSync=false,sessions=offlineSessions(it.sessions)) }
        if (wanted) { val wait = (1000L shl retry.coerceAtMost(5)).coerceAtMost(30000); retry++; main.postDelayed({ if (wanted && retryGeneration == generation) open() }, wait) }
    }
    private fun receive(event: JSONObject) {
        try {
            when (event.optString("type")) {
                "hello" -> { check(event.getJSONObject("data").optString("device").isNotBlank()); val stableGeneration=generation;main.postDelayed({if(stableGeneration==generation && mutable.value.connected)retry=0},30000); update { it.copy(connected = true, status = "Connected", sharedReadSync = event.getJSONObject("data").optJSONArray("capabilities")?.let { caps -> (0 until caps.length()).any { caps.optString(it)=="chat.read" } } == true) }; refresh(); mutable.value.thread?.let { subscribe(it,reconnecting=true) }; mutable.value.pendingOperations.keys.toList().forEach { checkPending(it) } }
                "result" -> calls.remove(event.optString("id"))?.invoke(event)
                "error" -> { val operation=event.optString("id") in mutable.value.pendingOperations || event.optString("id") in receiptQueries; calls.remove(event.optString("id"))?.invoke(event); if(operation)return; val history=event.optString("id")==historyCall; update { it.copy(error = event.optString("error"), historyLoading=if(history) false else it.historyLoading) } }
                "sessions.error" -> {
                    val data=event.getJSONObject("data")
                    if(data.optString("subscription")==listSubscription) update { it.copy(error=data.optString("error"),sessions=it.sessions.map { row -> JSONObject(row.toString()).put("status","unknown").put("live",false).put("unread",JSONObject.NULL) }) }
                }
                "sessions.page" -> {
                    val data=event.getJSONObject("data")
                    if(data.optString("subscription")==listSubscription) sessionPage(data.getJSONObject("page"))
                }
                "chat.state" -> {
                    val data = event.getJSONObject("data"); if (data.optString("id") != mutable.value.thread) return
                    val next = sentMessageState(synchronizedChatState(mutable.value, data)).let { if(data.optJSONObject("snapshot")!=null) it.copy(cached=!data.optBoolean("connected") && !data.optBoolean("archived")) else it }
                    if (next === mutable.value) return
                    if(data.optJSONObject("snapshot")!=null && next.cached)reading.memory.remember(next.copy(cached=false))
                    if(next.snapshot?.optBoolean("hasUnreadTurn")==false) readAttempt=""
                    update { next }
                    val attempt="$subscription:${next.desktopConnected}:${next.threadArchived}:${next.snapshot?.optJSONArray("turns")?.let { it.optJSONObject(it.length()-1)?.optString("turnId") }}"
                    if ((next.desktopConnected || next.threadArchived) && !next.historyLoading && next.historyPage==null && historyAttempt!=attempt) { historyAttempt=attempt; history(navigate=false) }
                }
            }
        } catch (_: Exception) { update { it.copy(error = "服务端响应不受支持，请更新或重新连接") } }
    }
    private fun command(method: String, fields: JSONObject = JSONObject(), id: String = UUID.randomUUID().toString(), result: (JSONObject) -> Unit = {}): Boolean {
        if (!mutable.value.connected) { update { it.copy(error = "请先连接一台电脑") }; return false }
        fields.put("id", id).put("method", method); calls[id] = result
        if(method=="chat.subscribe")stream.reset(fields.optString("thread"),id)
        if (socket?.send(fields.toString()) != true) { calls.remove(id); update { it.copy(error = "未发送，请先检查待确认操作再重试") }; return false }
        return true
    }
    private fun sessionPage(page: JSONObject) {
        val rows=objects(page.optJSONArray("items"))
        update { s -> s.copy(sessionSync=page.optJSONObject("sync"),sessions=rows, projects=objects(page.optJSONArray("projects")), hasMore=page.optBoolean("hasMore"),
            threadArchived=rows.firstOrNull { it.optString("id")==s.thread }?.optBoolean("archived") ?: s.threadArchived,
            snapshot=rows.firstOrNull { it.optString("id")==s.thread && it.optString("readToken")==s.readToken && s.readToken.isNotBlank() && it.has("unread") && !it.isNull("unread") }?.let { row ->
                if(!row.optBoolean("unread")) readAttempt=""
                updatedUnread(s.snapshot,row.optBoolean("unread"))
            } ?: s.snapshot) }
    }
    fun markRead(latestReplyVisible: Boolean) {
        val s=mutable.value
        if(!readReceiptEligible(s,Runtime.visible,latestReplyVisible)) return
        val key="${s.current?.id}:$generation:$subscription:${s.thread}:${s.readToken}"
        if(readAttempt==key) return
        readAttempt=key
        command("chat.read",JSONObject().put("thread",s.thread).put("readToken",s.readToken)) { event ->
            if(event.optJSONObject("data")?.optBoolean("accepted")!=true && readAttempt==key) readAttempt=""
        }
    }
    fun refresh() {
        val s=mutable.value; val ticket=UUID.randomUUID().toString();listSubscription=ticket
        val fields=JSONObject().put("archived",s.archived).put("project",s.project).put("text",s.sessionSearch).put("limit",s.sessionLimit)
        command("sessions.browse",JSONObject(fields.toString())) { event ->
            if(ticket==listSubscription) {
                event.optJSONObject("data")?.let(::sessionPage)
                command("sessions.subscribe",fields,ticket)
            }
        }
    }
    fun sessionFilter(search: String=mutable.value.sessionSearch, archived: Boolean=mutable.value.archived, project: String=mutable.value.project) {
        update { it.copy(sessionSearch=search, archived=archived, project=project, sessionLimit=100) };refresh()
    }
    fun moreSessions() { update { it.copy(sessionLimit=(it.sessionLimit+100).coerceAtMost(5000)) };refresh() }
    fun createChat(project: String) { mutation("sessions.create",JSONObject().put("project",project)) { result ->
        update { it.copy(archived=false,project="",sessionSearch="",sessionLimit=100) }
        result.optString("id").takeIf { it.isNotBlank() }?.let { subscribe(it) };refresh()
        if(!result.optBoolean("desktopReady")&&!result.optBoolean("opening")) reportError("会话已创建，但尚未确认官方桌面接管。请在电脑上的 Codex 打开该会话；连接后将自动同步，不要重复创建。")
    } }
    fun renameChat(name: String) {
        val thread=mutable.value.thread ?: return
        mutation("sessions.rename",JSONObject().put("thread",thread).put("text",name)) { result ->
            val title=result.optString("title")
            if(title.isNotBlank()) update { s -> s.copy(sessions=s.sessions.map { row -> if(row.optString("id")==thread) jsonCopy(row).put("title",title) else row }) }
            refresh()
        }
    }
    fun archiveChat(id: String, archived: Boolean) { mutation(if(archived) "sessions.archive" else "sessions.unarchive",JSONObject().put("thread",id)) {
        if(mutable.value.thread==id) leaveChat();refresh()
    } }
    fun subscribe(thread: String, reconnecting: Boolean = false) {
        if(thread!=mutable.value.thread) goalMode(false)
        val changed = mutable.value.thread != thread
        val ticket = ++subscription
        historyAttempt="";historyCall=null
        mutable.value.current?.let { store.put("thread:${it.id}", thread) }
        update { it.copy(historyPage=if(reconnecting) it.historyPage else null, historyIndex=if(reconnecting) it.historyIndex else null, historyLoading=false, historyTarget=null, sendUserBefore=null, thread = thread, threadArchived = it.sessions.firstOrNull { row -> row.optString("id")==thread }?.optBoolean("archived") ?: false, sequence = if (changed) -1 else it.sequence, readToken = if (changed) "" else it.readToken, snapshot = if (changed) null else it.snapshot,cached=changed || it.cached,
            desktopConnected = if (changed) false else it.desktopConnected,
            catalog = if (changed) null else it.catalog, skills = savedSkills(it.current?.id, thread)) }
        if(changed) restoreCached()
        if(!mutable.value.connected)return
        command("chat.subscribe", JSONObject().put("thread", thread).put("stream",true))
        command("chat.catalog", JSONObject().put("thread", thread)) { event ->
            if (ticket == subscription && mutable.value.thread == thread) update { it.copy(catalog = event.optJSONObject("data")) }
        }
    }
    fun history(cursor: String? = "latest", offset: Int = 0, navigate: Boolean = true, extend: Boolean = false) {
        if(extend && mutable.value.historyLoading) return
        val current=mutable.value.historyPage ?: mutable.value.historyIndex
        val thread=mutable.value.thread ?: return
        if(!mutable.value.connected) { if(cursor=="latest")update { it.copy(historyPage=null,historyTarget="cached-latest:${UUID.randomUUID()}") };return }
        val ticket=subscription;val gen=generation;val call=UUID.randomUUID().toString();historyCall=call
        update { it.copy(historyLoading=true,sendUserBefore=if(navigate) null else it.sendUserBefore) }
        val fields=JSONObject().put("thread",thread).put("lazy",true).put("limit",20).put("offset",offset).put("indexOnly",!navigate && cursor=="latest" && mutable.value.desktopConnected && mutable.value.snapshot?.optString("historySource")!="rollout")
        if(cursor!=null) fields.put("cursor",cursor)
        command("chat.history",fields,call) { event ->
            if(ticket!=subscription || gen!=generation || thread!=mutable.value.thread || call!=historyCall) return@command
            val page=event.optJSONObject("data")
            if(page?.optJSONArray("turns")==null || page.optJSONArray("questions")==null) {update { it.copy(historyLoading=false,error="请更新网关以浏览完整历史。") };return@command}
            update { state ->
                val snapshot=state.snapshot?.let { original ->
                    if(cursor=="latest" && original.optString("historySource")=="rollout") jsonCopy(original).put("turns",page.getJSONArray("turns")) else original
                }
                seedHistoryIndex(page,snapshot,fields.optBoolean("indexOnly"))
                state.copy(snapshot=snapshot,historyIndex=page,historyLoading=false,
                    historyPage=if(navigate) if(cursor=="latest") null else if(extend) extendHistory(current,page) else page else state.historyPage,
                    historyTarget=if(navigate && !extend) "${cursor ?: offset}:$call" else state.historyTarget)
            }
        }
        main.postDelayed({if(historyCall==call && mutable.value.historyLoading) { calls.remove(call); update {it.copy(historyLoading=false,error="历史读取超时，请重试。")}}},30000)
    }
    suspend fun item(message: Message): Message = kotlinx.coroutines.withTimeout(30000) {
        val state=mutable.value;val computer=state.current ?: error("Disconnected")
        val thread=state.thread ?: error("No conversation")
        val key="${readingScope(computer)}:$thread:${message.id}:${message.bodyRevision}"
        bodyCache[key]?.let { return@withTimeout it }
        kotlinx.coroutines.suspendCancellableCoroutine<Message> { continuation ->
            val call=UUID.randomUUID().toString()
            continuation.invokeOnCancellation {main.post {calls.remove(call)}}
            command("chat.item",JSONObject().put("thread",thread).put("turn",message.turn).put("item",message.id.removePrefix(message.turn+":")),call) { event ->
                if(!continuation.isActive)return@command
                val data=event.optJSONObject("data")
                if(event.optString("type")=="error" || data==null || mutable.value.current!=computer || mutable.value.thread!=thread) {
                    continuation.resumeWith(Result.failure(IllegalStateException("Detail read failed")))
                }else {
                    val detail=message.copy(text=data.optString("text"),command=data.optString("command"),deferred=false,bodyRevision=data.optString("bodyRevision"))
                    bodyCache[key]=detail
                    while(bodyCache.size>4 || bodyCache.size>1 && bodyCache.values.sumOf {it.text.length}>8000000)bodyCache.remove(bodyCache.keys.first())
                    continuation.resumeWith(Result.success(detail))
                }
            }
        }
    }
    suspend fun image(message: Message,image: MessageImage): ByteArray = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
        val selected=mutable.value;val computer=selected.current?:error("Disconnected");val thread=selected.thread?:error("No conversation")
        require(selected.connected&&!selected.cached)
        val body=JSONObject().put("thread",thread).put("turn",message.turn).put("item",message.id.removePrefix(message.turn+":")).put("image",image.id).toString().toRequestBody("application/json".toMediaType())
        val request=Request.Builder().url(computer.origin+"/api/v1/chat/image").header("Origin",computer.origin).header("Authorization","Bearer ${computer.token}").post(body).build()
        client.newBuilder().callTimeout(30,TimeUnit.SECONDS).readTimeout(30,TimeUnit.SECONDS).build().newCall(request).execute().use {response ->
            require(response.code==200 && response.header("Content-Type") in listOf("image/png","image/jpeg","image/gif","image/webp"))
            val bytes=response.body?.byteStream()?.use {input ->
                val out=java.io.ByteArrayOutputStream();val buffer=ByteArray(32768)
                while(out.size()<=MAX_ATTACHMENT_SIZE){val n=input.read(buffer,0,minOf(buffer.size,MAX_ATTACHMENT_SIZE.toInt()+1-out.size()));if(n<0)break;out.write(buffer,0,n)};out.toByteArray()
            }?:error("Image unavailable")
            require(bytes.size in 1..MAX_ATTACHMENT_SIZE.toInt() && mutable.value.current==computer && mutable.value.thread==thread)
            bytes
        }
    }
    private fun savedSkills(computer: String?, thread: String?): Set<String> = try {
        val array = JSONArray(store.get("skills:$computer:$thread") ?: "[]")
        (0 until array.length()).map { array.getString(it) }.toSet()
    } catch (_: Exception) { emptySet() }
    fun skill(id: String, selected: Boolean) {
        val s = mutable.value
        val next = if (selected) s.skills + id else s.skills - id
        if (next.size > 8) { reportError("一次最多选择 8 个 Skill。"); return }
        store.put("skills:${s.current?.id}:${s.thread}", JSONArray(next.toList()).toString())
        update { it.copy(skills = next) }
    }
    fun settings(options: JSONObject, expected: String? = null) {
        val snapshot = mutable.value.snapshot ?: return
        options.put("expected", expected ?: snapshot.optString("controlsRevision"))
        mutation("chat.settings", JSONObject().put("thread", mutable.value.thread).put("options", options)) {
            if (it.optBoolean("applied", true)) mutable.value.thread?.let(::subscribe)
            else reportError("官方桌面设置发生了变化，未应用；请重新打开设置后再修改。")
        }
    }
    fun leaveChat() { goalMode(false); subscription++; mutable.value.current?.let { store.put("thread:${it.id}", null) }; update { it.copy(thread = null, snapshot = null, historyPage=null,historyIndex=null,historyLoading=false,historyTarget=null,desktopConnected = false) } }
    fun draft(value: String) { val s = mutable.value; store.put("draft:${s.current?.id}:${s.thread}", value) }
    fun draft(): String { val s = mutable.value; return store.get("draft:${s.current?.id}:${s.thread}").orEmpty() }
    fun attachmentDraft(): List<JSONObject> {val s=mutable.value;return objects(JSONArray(store.get("attachments:${s.current?.id}:${s.thread}")?:"[]"))}
    fun removeAttachment(id: String) {val s=mutable.value;if(s.pending!=null)return;store.put("attachments:${s.current?.id}:${s.thread}",JSONArray(attachmentDraft().filter {it.optString("id")!=id}).toString());update {it.copy(draftRevision=it.draftRevision+1)}}
    fun upload(uri: android.net.Uri) {
        val s=mutable.value;val computer=s.current?:return;val thread=s.thread?:return
        if(!s.connected||!s.desktopConnected||s.cached||s.threadArchived||s.pending!=null||s.uploadBusy||s.goalDraft)return
        val key="attachments:${computer.id}:$thread"
        if(attachmentDraft().size>=8){reportError("每条消息最多 8 个附件。");return}
        update {it.copy(uploadBusy=true)}
        Thread {
            try {
                val response=uploadAttachment(client,app.contentResolver,computer,thread,uri)
                main.post {
                    val files=objects(JSONArray(store.get(key)?:"[]"));store.put(key,JSONArray(files+response).toString())
                    update {it.copy(uploadBusy=false,draftRevision=it.draftRevision+1)}
                }
            } catch (error:Exception) {val message=attachmentUploadFailure((error as? AttachmentUploadException)?.status?:0);main.post {update {it.copy(uploadBusy=false,error=message)}}}
        }.start()
    }
    fun notifications(enabled: Boolean) {
        val thread=mutable.value.thread?:return
        mutation("chat.notifications",JSONObject().put("thread",thread).put("enabled",enabled)) {result ->
            update {if(it.thread==thread)it.copy(snapshot=it.snapshot?.let {raw -> jsonCopy(raw).put("notificationPreference",result)}) else it}
        }
    }
    fun goalAction(action: String) {
        val s=mutable.value
        mutation("chat.goal",JSONObject().put("thread",s.thread).put("options",JSONObject().put("goalAction",action).put("expectedGoal",s.snapshot?.optString("goalRevision")))) { if(mutable.value.thread==s.thread) s.thread?.let { subscribe(it,reconnecting=true) } }
    }
    fun goalMode(selected: Boolean) { update { it.copy(goalDraft=selected) } }
    fun mutate(method: String, text: String = "", request: String? = null, decision: String? = null) {
        val actualMethod=if(method=="chat.send" && mutable.value.goalDraft) "chat.goal" else method
        val fields = JSONObject().put("thread", mutable.value.thread).put("text", text); if (request != null) fields.put("requestId", request); if (decision != null) fields.put("decision", decision)
        val s = mutable.value
        val key = "draft:${s.current?.id}:${s.thread}"
        if(method in listOf("chat.send","chat.steer","chat.queue")) {
            if(s.uploadBusy){reportError("请等待附件上传完成。");return}
            val ids=attachmentDraft().map {it.getString("id")}
            if(actualMethod=="chat.goal" && ids.isNotEmpty()){reportError("请先移除附件，再设置目标。");return}
            fields.put("attachments",JSONArray(ids))
        }
        if (method in listOf("chat.send", "chat.steer", "chat.goal", "chat.queue")) fields.put("skills", JSONArray(s.skills.toList()))
        if(actualMethod=="chat.goal") fields.put("options",JSONObject().put("expectedGoal",s.snapshot?.optString("goalRevision")))
        mutation(actualMethod, fields) {
            if(mutable.value.thread==s.thread) goalMode(false)
            if (method in listOf("chat.send", "chat.steer", "chat.goal", "chat.queue") && store.get(key) == text) {
                store.put(key, null)
                update { it.copy(draftRevision = it.draftRevision + 1) }
            }
        }
    }
    private fun mutation(method: String, fields: JSONObject, after: (JSONObject) -> Unit = {}) {
        val s = mutable.value; val c = s.current ?: return
        if (!s.connected || (!method.startsWith("sessions.") && (!s.desktopConnected || s.snapshot == null || s.threadArchived || s.cached))) { reportError("请先连接一台电脑"); return }
        if (s.pendingFor(fields.optString("thread").ifBlank { null }) != null) return
        val id = UUID.randomUUID().toString()
        val operation=JSONObject().put("id",id).put("method",method).put("thread",fields.optString("thread")).put("status","dispatched")
        savePending(s.pendingOperations + (id to operation))
        update { it.copy(operationFailures=it.operationFailures-fields.optString("thread")) }
        if(fields.optJSONArray("attachments")?.length()?.let {it>0}==true)store.put("pending-files:${c.id}:$id",JSONObject().put("id",id).put("key","attachments:${c.id}:${s.thread}").put("ids",fields.getJSONArray("attachments")).toString())
        if(method in listOf("chat.send","chat.steer")) {
            historyCall=UUID.randomUUID().toString()
            update { it.copy(pending=id,historyPage=null,historyLoading=false,historyTarget="sent:$id",sendScrollRevision=it.sendScrollRevision+1,sendUserBefore=latestUserKey(it.snapshot)) }
        } else update { it.copy(pending = id) }
        val queued=command(method, fields, id) {
            val data=it.optJSONObject("data")
            if(id in mutable.value.pendingOperations && data?.optString("id")==id) {
                receipt(it)
                if(data.optString("status")=="confirmed") after(data.optJSONObject("result") ?: JSONObject())
            } else if(id in mutable.value.pendingOperations) {
                receipt(JSONObject().put("data",JSONObject().put("id",id).put("status","unknown").put("result",JSONObject().put("error",it.optString("error")))))
            }
        }
        // OkHttp false means this message was not enqueued, not an uncertain send.
        if(!queued) receipt(JSONObject().put("data",JSONObject().put("id",id).put("status","not_sent")))
    }
    private fun loadPending(computer: Computer) = pendingOperations(store.get("pending-operations:${computer.id}"),store.get("pending:${computer.id}"))
    fun hasPending(computer: Computer): Boolean = loadPending(computer).isNotEmpty()
    private fun savePending(operations: Map<String,JSONObject>) {
        val c=mutable.value.current ?: return
        store.put("pending-operations:${c.id}",JSONArray(operations.values.toList()).toString())
        // Write the new journal before removing the legacy slot.
        store.put("pending:${c.id}",null)
        update { it.copy(pendingOperations=operations) }
    }
    private fun receipt(event: JSONObject) {
        val data=event.optJSONObject("data") ?: return
        val id=data.optString("id");val operation=mutable.value.pendingOperations[id] ?: return
        val c=mutable.value.current ?: return
        val filesKey=if(operation.optBoolean("legacy")) "pending-files:${c.id}" else "pending-files:${c.id}:$id"
        if(data.optString("status")=="confirmed") {
            val files=store.get(filesKey)?.let(::JSONObject)
            if(files?.optString("id")==id) {
                val ids=files.getJSONArray("ids");val key=files.getString("key")
                val keep=objects(JSONArray(store.get(key)?:"[]")).filter {file -> (0 until ids.length()).none {ids.optString(it)==file.optString("id")}}
                store.put(key,JSONArray(keep).toString());update {it.copy(draftRevision=it.draftRevision+1)}
            }
        }
        if(settlesPending(id,data)) {
            savePending(mutable.value.pendingOperations-id)
            store.put(filesKey,null)
            operationFailure(operation,data)?.let { notice -> update { it.copy(operationFailures=it.operationFailures + (notice.optString("thread") to notice)) } }
        } else savePending(mutable.value.pendingOperations + (id to jsonCopy(operation).put("status",data.optString("status")).put("error",data.optJSONObject("result")?.optString("error").orEmpty())))
    }
    fun dismissOperation(id: String) { update { it.copy(operationFailures=it.operationFailures.filterValues { notice -> notice.optString("id")!=id }) } }
    private fun queryReceipt(method: String, id: String?) {
        if(id == null || id !in mutable.value.pendingOperations)return
        val call=UUID.randomUUID().toString();receiptQueries[call]=id
        val queued=command(method,JSONObject().put("requestId",id).put("text","inspected"),call) { event ->
            receiptQueries.remove(call)
            if(event.optJSONObject("data")!=null) receipt(event)
            else mutable.value.pendingOperations[id]?.let { operation ->
                savePending(mutable.value.pendingOperations + (id to jsonCopy(operation).put("error",event.optString("error"))))
            }
        }
        if(!queued) receiptQueries.remove(call)
    }
    fun checkPending(id: String? = mutable.value.pending) = queryReceipt("operation.get",id)
    fun acknowledgePending(id: String? = mutable.value.pending) = queryReceipt("operation.acknowledge",id)
}
