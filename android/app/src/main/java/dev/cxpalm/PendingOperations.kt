package dev.cxpalm

import org.json.JSONArray
import org.json.JSONObject

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
