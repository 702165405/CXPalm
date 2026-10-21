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
