package dev.cxpalm

import org.json.JSONObject
import org.json.JSONArray
import java.util.Locale

data class ContentPreview(val text: String, val truncated: Boolean)
fun contentPreview(text: String, maxChars: Int = 64000, maxLines: Int = 400, maxColumns: Int = 1000): ContentPreview {
    val out = StringBuilder()
    var truncated = false
    val lines = text.lineSequence().iterator()
    var count = 0
    while (lines.hasNext()) {
        if (count++ >= maxLines) { truncated = true; break }
        val line = lines.next()
        if (count > 1) out.append('\n')
        val room = (maxChars - out.length).coerceAtLeast(0)
        val shown = line.take(minOf(room, maxColumns))
        out.append(shown)
        if (shown.length < line.length) truncated = true
        if (out.length >= maxChars) { truncated = truncated || lines.hasNext(); break }
    }
    return ContentPreview(out.toString(), truncated)
}

fun fileSize(bytes: Long): String = when {
    bytes >= 1024 * 1024 -> String.format(Locale.ROOT, "%.1f MB", bytes / (1024.0 * 1024))
    bytes >= 1024 -> String.format(Locale.ROOT, "%.1f KB", bytes / 1024.0)
    else -> "$bytes B"
}

fun approvalDecisions(request: JSONObject): List<String> {
    if(request.optString("method")=="mcpServer/elicitation/request") {
        val params=request.optJSONObject("params") ?: return emptyList()
        val schema=params.optJSONObject("requestedSchema") ?: return emptyList()
        if(params.optString("mode")!="form" || params.optJSONObject("_meta")?.optString("codex_approval_kind")!="mcp_tool_call" || schema.optString("type")!="object" || schema.optJSONObject("properties")?.length()!=0) return emptyList()
        if(schema.keys().asSequence().any {it !in listOf("type","properties","required","title","description")}) return emptyList()
        if(schema.has("required") && schema.optJSONArray("required")?.length()!=0) return emptyList()
        val persist=params.getJSONObject("_meta").opt("persist")
        val session=persist=="session" || persist is JSONArray && (0 until persist.length()).any {persist.opt(it)=="session"}
        return listOf("accept")+(if(session) listOf("acceptForSession") else emptyList())+listOf("decline","cancel")
    }
    if(request.optString("method")=="item/permissions/requestApproval") {
        val permissions=request.optJSONObject("params")?.optJSONObject("permissions") ?: return emptyList()
        return if(permissions.keys().asSequence().all {it in listOf("network","fileSystem")}) listOf("accept","decline") else listOf("decline")
    }
    if(request.optString("method") !in listOf("item/commandExecution/requestApproval","item/fileChange/requestApproval")) return emptyList()
    val params=request.optJSONObject("params")
    if(params?.has("availableDecisions")!=true || params.isNull("availableDecisions")) return listOf("accept","decline")
    val available=params.optJSONArray("availableDecisions") ?: return emptyList()
    return listOf("accept","acceptForSession","decline","cancel").filter { choice -> (0 until available.length()).any {available.opt(it)==choice} }
}

fun approvalSummary(request: JSONObject, missing: String = "请在官方 Codex 桌面查看完整请求。", empty: String = "文件变更或工具请求，请展开详情并核对后再批准。"): String {
    val params = request.optJSONObject("params") ?: return missing
    return listOf("message", "command", "reason", "cwd").mapNotNull { key -> params.opt(key)?.toString()?.takeIf { it.isNotBlank() && it != "null" } }
        .joinToString("\n").take(3000).ifBlank { empty }
}

fun sessionControlValues(snapshot: JSONObject?): Map<String,String> {
    fun value(obj: JSONObject?,key: String)=obj?.opt(key)?.takeUnless { it==JSONObject.NULL }?.toString().orEmpty()
    val settings=snapshot?.optJSONObject("latestThreadSettings") ?: snapshot?.optJSONObject("currentPermissions")
    val profile=value(settings?.optJSONObject("activePermissionProfile"),"id").ifBlank { value(settings,"permissions") }.ifBlank { value(settings?.optJSONObject("sandboxPolicy"),"type") }
    val permission=if(profile in listOf(":workspace","workspace","workspace-write") && value(settings,"approvalsReviewer")=="auto_review") "auto-review" else when(profile) { ":read-only","read-only" -> "read-only"; ":workspace","workspace","workspace-write" -> "workspace"; ":danger-full-access","danger-full-access","full" -> "full";else -> "" }
    return mapOf("model" to value(snapshot,"latestModel").ifBlank { value(settings,"model") },"effort" to value(snapshot,"latestReasoningEffort").ifBlank { value(settings,"effort") }.ifBlank { "default" },"permission" to permission,"mode" to value(snapshot?.optJSONObject("latestCollaborationMode"),"mode").ifBlank { "default" })
}
// Same source fields as the Web context ring; absent or zero windows stay unknown.
data class ContextUsage(val used: Long, val limit: Long, val percent: Int)
fun contextUsage(snapshot: JSONObject?): ContextUsage? {
    val info = snapshot?.optJSONObject("latestTokenUsageInfo") ?: return null
    val used = (info.optJSONObject("last")?.opt("inputTokens") as? Number)?.toDouble() ?: return null
    val limit = (info.opt("modelContextWindow") as? Number)?.toDouble() ?: return null
    if (used.isNaN() || limit.isNaN() || limit <= 0) return null
    return ContextUsage(used.toLong(), limit.toLong(), Math.round(used / limit * 100).toInt().coerceIn(0, 100))
}
fun changedControlSettings(base: Map<String,String>,draft: Map<String,String>,group: String?): Map<String,String> {
    val fields=when(group) {"model" -> listOf("model","effort");"permission" -> listOf("permission");"plan" -> listOf("mode");else -> emptyList()}
    return fields.mapNotNull { key -> draft[key]?.takeIf { it.isNotBlank() && it!=base[key] }?.let { key to it } }.toMap()
}


// The single history-controls item precedes non-empty message lists.
fun replyBottomReached(replyIndex: Int, firstVisibleItemIndex: Int, visibleReplyBottom: Int?, viewportEnd: Int): Boolean =
    replyIndex >= 0 && (firstVisibleItemIndex > replyIndex + 1 ||
        visibleReplyBottom != null && visibleReplyBottom <= viewportEnd + 8)

fun showLatestContent(hasMessages: Boolean, showingHistory: Boolean, canScrollForward: Boolean): Boolean =
    hasMessages && (showingHistory || canScrollForward)

fun appBackDestination(page: String): String = if (page == "computers") "settings" else "sessions"

fun activeQuestionIndex(questionTurns: List<String>, messages: List<Message>, firstMessage: Int, atBottom: Boolean, latest: Boolean): Int {
    if (questionTurns.isEmpty()) return -1
    if (atBottom && latest) return questionTurns.lastIndex
    val users = messages.withIndex().filter { it.value.role == "You" }
    val row = if (atBottom) users.lastOrNull() else users.lastOrNull { it.index <= firstMessage } ?: users.firstOrNull()
    return questionTurns.indexOf(row?.value?.turn)
}

internal fun questionAtPosition(questions: List<JSONObject>, position: Float): String? =
    if (!position.isFinite() || position < 0) null
    else questions.getOrNull(position.toInt())?.optString("turn")?.takeIf { it.isNotBlank() }

fun computerConnectionState(state: ViewState, computer: Computer, thread: String?, pending: String?, skills: Set<String>): ViewState {
    val same = state.current?.id == computer.id
    return state.copy(operationFailures=if(same) state.operationFailures else emptyMap(), current = computer, sessions = if (same) state.sessions else emptyList(), projects = if (same) state.projects else emptyList(),
        archived = same && state.archived, project = if (same) state.project else "", sessionSearch = if (same) state.sessionSearch else "", sessionLimit = if (same) state.sessionLimit else 100,
        threadArchived = same && state.threadArchived, thread = thread, snapshot = if (same) state.snapshot else null, pending = pending, connected = false, desktopConnected = false,
        historyPage = if (same) state.historyPage else null, historyIndex = if (same) state.historyIndex else null, historyLoading = false, historyTarget = if (same) state.historyTarget else null,
        sendUserBefore = if(same) state.sendUserBefore else null, readToken = if (same) state.readToken else "", sharedReadSync = false, sequence = -1, error = null,
        catalog = if (same) state.catalog else null, skills = skills)
}

fun mergeHistoryTurns(before: List<JSONObject>, after: List<JSONObject>): List<JSONObject> {
    val turns = linkedMapOf<String, JSONObject>()
    (before + after).forEach { turns[it.optString("turnId").ifBlank { it.optString("id") }] = it }
    return turns.values.toList()
}
fun extendHistory(current: JSONObject?, page: JSONObject): JSONObject {
    if(current == null) return page
    val earlier=page.optInt("offset") < current.optInt("offset")
    val old=objects(current.optJSONArray("turns")); val fresh=objects(page.optJSONArray("turns"))
    return jsonCopy(page).put("offset", minOf(current.optInt("offset"),page.optInt("offset")))
        .put("hasMore",if(earlier) current.optBoolean("hasMore") else page.optBoolean("hasMore"))
        .put("turns",org.json.JSONArray(if(earlier) mergeHistoryTurns(fresh,old) else mergeHistoryTurns(old,fresh)))
}
fun historyDisplay(page: JSONObject?, snapshot: JSONObject?): JSONObject? = when {
    page == null -> snapshot
    page.optBoolean("hasMore") -> page
    else -> {
        val latest=objects(snapshot?.optJSONArray("turns"))
        val start=((snapshot?.optInt("historyTotal",page.optInt("total")) ?: page.optInt("total"))-latest.size).coerceAtLeast(0)
        val suffix=latest.drop((page.optInt("offset")-start).coerceAtLeast(0))
        jsonCopy(page).put("turns",org.json.JSONArray(mergeHistoryTurns(objects(page.optJSONArray("turns")),suffix)))
    }
}
fun latestUserKey(snapshot: JSONObject?): String = snapshot?.let(::conversationMessages)?.lastOrNull { it.role == "You" }?.let { "${it.id}:${it.text}" }.orEmpty()
fun sentMessageState(state: ViewState): ViewState = if(state.sendUserBefore != null && latestUserKey(state.snapshot) != state.sendUserBefore)
    state.copy(sendUserBefore=null,sendScrollRevision=state.sendScrollRevision+1) else state

fun sessionIndicator(status: String, unread: Boolean?, archived: Boolean = false): String? = when {
    archived -> null
    status == "running" || status == "waiting" -> status
    unread != true -> null
    status == "failed" || status == "interrupted" -> status
    else -> "completed"
}
fun effortLabel(value: String): String = when(value) {
    "default" -> tr("模型默认"); "minimal" -> tr("最少"); "low" -> tr("低")
    "medium" -> tr("中"); "high" -> tr("高"); "xhigh" -> tr("超高"); else -> value
}

fun messageSendMethod(active: Boolean, hasQueue: Boolean): String = if(active || hasQueue) "chat.queue" else "chat.send"

fun seedHistoryIndex(page: JSONObject, snapshot: JSONObject?, indexOnly: Boolean): JSONObject =
    if(indexOnly) page.put("turns",snapshot?.optJSONArray("turns") ?: JSONArray()) else page

internal fun jsonCopy(source: JSONObject): JSONObject = JSONObject().apply {
    source.keys().forEach { key -> put(key, source.opt(key)) }
}

internal fun updatedUnread(snapshot: JSONObject?, unread: Boolean): JSONObject? =
    if(snapshot == null || snapshot.optBoolean("hasUnreadTurn") == unread) snapshot
    else jsonCopy(snapshot).put("hasUnreadTurn", unread)

internal fun shouldFollowStream(following: Boolean, latest: Boolean, dragging: Boolean, scrolling: Boolean): Boolean =
    following && latest && !dragging && !scrolling

internal data class RenderedConversation(val messages: List<Message>, val timings: Map<String,TurnTiming>, val footers: Set<Int>, val sourceSnapshot: JSONObject?=null, val sourcePage: JSONObject?=null)
internal fun renderConversationData(page: JSONObject?, snapshot: JSONObject?): RenderedConversation {
    val display=historyDisplay(page,snapshot)
    val messages=display?.let(::conversationMessages).orEmpty()
    val timings=objects(display?.optJSONArray("turns")).associate { it.optString("turnId").ifBlank { it.optString("id") } to turnTiming(it) }
    return RenderedConversation(messages,timings,timingFooterIndices(messages),snapshot,page)
}

internal fun detailRefreshKey(message: Message): String = if(message.deferred && message.status=="inProgress") "" else message.bodyRevision
