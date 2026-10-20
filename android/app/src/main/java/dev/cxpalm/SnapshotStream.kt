package dev.cxpalm

import org.json.JSONObject
import org.json.JSONArray

internal class SnapshotStream {
    private var expectedThread: String?=null
    private var expectedID: String?=null
    private var current: JSONObject?=null
    private var revision=0L
    private var broken=false
    @Synchronized fun reset(thread: String?=null,id: String?=null) {expectedThread=thread;expectedID=id;current=null;revision=0;broken=false}
    @Synchronized fun accept(event: JSONObject): JSONObject? {
        val type=event.optString("type")
        if(type!="chat.state" && type!="chat.delta")return event
        val data=event.getJSONObject("data")
        if(expectedThread!=null && data.optString("id")!=expectedThread || data.has("streamID") && expectedID!=null && data.optString("streamID")!=expectedID)return null
        if(type=="chat.state") {current=data;revision=data.optLong("streamRevision");broken=false;return event}
        if(broken)return null
        try {
            check(current!=null && data.optLong("base",-1)==revision && data.optLong("streamRevision",-1)==revision+1)
            var result: Any=current!!
            objects(data.optJSONArray("ops")).forEach { result=apply(result,it) }
            val value=jsonCopy(result as JSONObject).put("streamRevision",data.getLong("streamRevision"))
            current=value;revision=data.getLong("streamRevision")
            return JSONObject().put("type","chat.state").put("data",value)
        }catch(error: Exception) {broken=true;throw error}
    }
    private fun apply(root: Any,operation: JSONObject): Any {
        val path=operation.getJSONArray("path");require(path.length()<=24)
        val action=operation.getString("op");val content=operation.opt("value") ?: JSONObject.NULL
        fun visit(value: Any?,depth: Int): Any {
            if(depth==path.length())return when(action) {
                "set" -> content
                "append" -> {require(value is String && content is String);value+content}
                "resize" -> {require(value is JSONArray && content is Number);val size=content.toInt();require(size in 0..value.length());JSONArray().apply {repeat(size) {put(value.opt(it))}}}
                else -> error("invalid operation")
            }
            val key=path.getString(depth)
            if(value is JSONObject) {
                val copy=jsonCopy(value)
                if(depth==path.length()-1 && action=="remove")copy.remove(key)
                else copy.put(key,visit(value.opt(key),depth+1))
                return copy
            }
            require(value is JSONArray && key.matches(Regex("0|[1-9][0-9]*")))
            val index=key.toInt();require(index in 0..value.length() && !(depth==path.length()-1 && action=="remove"))
            val copy=JSONArray().apply {repeat(value.length()) {put(value.opt(it))}}
            copy.put(index,visit(value.opt(index),depth+1));return copy
        }
        return visit(root,0)
    }
}
