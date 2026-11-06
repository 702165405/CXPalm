package dev.cxpalm

import org.json.JSONArray
import org.json.JSONObject

internal fun operationLabel(method: String): String = tr(when(method) {
    "chat.send" -> "发送消息"
    "chat.steer" -> "引导"
    "chat.queue" -> "排队发送"
    "chat.queue.cancel" -> "移除排队消息"
    "chat.stop" -> "停止任务"
    "chat.approve" -> "审批操作"
    "chat.settings" -> "修改会话设置"
    "chat.goal" -> "修改会话目标"
    "chat.compact" -> "压缩上下文"
    "chat.notifications" -> "修改会话通知"
    "sessions.create" -> "新建会话"
    "sessions.rename" -> "重命名会话"
    "sessions.archive" -> "归档会话"
    "sessions.unarchive" -> "取消归档"
    else -> "待核对操作"
})
internal fun operationTitle(operation: JSONObject): String = operation.optString("title").trim().take(120).ifBlank {
    tr(if(operation.optString("method")=="sessions.create") "新会话" else "所属会话未记录")
}
internal fun operationTime(operation: JSONObject): String = operation.optLong("createdAtMs").takeIf {it>0}?.let {
    java.text.SimpleDateFormat("MM-dd HH:mm:ss",java.util.Locale.getDefault()).format(java.util.Date(it))
} ?: tr("操作时间未记录")

// Old Android versions saved only an ID. Keep that unfenced operation protected
// until inspected; never guess its conversation from the last viewed screen.
internal fun pendingOperations(saved: String?, legacy: String?): Map<String, JSONObject> {
    val operations = objects(JSONArray(saved ?: "[]")).associateBy { it.getString("id") }.toMutableMap()
    if (legacy != null && legacy !in operations) operations[legacy] = JSONObject().put("id",legacy).put("legacy",true).put("method","previous-version")
    return operations
}

internal fun ViewState.pendingFor(thread: String?): JSONObject? = pendingOperations.values.firstOrNull {
    it.optBoolean("legacy") || it.optString("thread").ifBlank { null } == thread
}

internal fun ViewState.operationNotice(thread: String?): JSONObject? = pendingFor(thread) ?: operationFailures[thread.orEmpty()]

internal fun operationFailure(operation: JSONObject, receipt: JSONObject): JSONObject? {
    if(receipt.optString("id")!=operation.optString("id") || receipt.optString("status") !in listOf("rejected","not_sent"))return null
    val result=receipt.optJSONObject("result")
    return jsonCopy(operation).put("legacy",false).put("status",receipt.optString("status"))
        .put("method",result?.optString("method")?.ifBlank { null } ?: operation.optString("method"))
        .put("thread",result?.optString("thread")?.ifBlank { null } ?: operation.optString("thread"))
        .put("error",result?.optString("error").orEmpty())
}
