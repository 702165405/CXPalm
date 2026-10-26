package dev.cxpalm

import java.io.File
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class SnapshotStreamTest {
    private fun normalize(value: Any?): Any? = when(value) {
        is JSONObject -> value.keys().asSequence().sorted().associateWith {normalize(value.opt(it))}
        is JSONArray -> (0 until value.length()).map {normalize(value.opt(it))}
        is Number -> value.toDouble()
        JSONObject.NULL -> null
        else -> value
    }
    @Test fun goContractReconstructsExactSnapshotsWithoutCopyingUnchangedBodies() {
        val file=generateSequence(File(System.getProperty("user.dir")!!)) {it.parentFile}.map {File(it,"internal/server/testdata/stream-contract.json")}.first {it.isFile}
        val frames=objects(JSONArray(file.readText()))
        val stream=SnapshotStream();stream.reset("thread","fixture-stream")
        var first:JSONObject?=null
        frames.forEachIndexed { index,frame ->
            val state=stream.accept(frame.getJSONObject("event"))!!.getJSONObject("data")
            assertEquals(normalize(frame.getJSONObject("expected")),normalize(state))
            val turns=state.getJSONObject("snapshot").getJSONArray("turns")
            if(index==0) first=turns.getJSONObject(0).getJSONArray("items").getJSONObject(0)
            if(index==1) assertSame(first,turns.getJSONObject(0).getJSONArray("items").getJSONObject(0))
        }
        assertNull(stream.accept(JSONObject("""{"type":"chat.delta","data":{"id":"thread","streamID":"old","base":5,"streamRevision":6,"ops":[]}}""")))
        try {stream.accept(JSONObject("""{"type":"chat.delta","data":{"id":"thread","streamID":"fixture-stream","base":8,"streamRevision":9,"ops":[]}}"""));fail("gap accepted")}catch(_:IllegalStateException){}
        assertNull(stream.accept(frames[1].getJSONObject("event")))
        stream.reset("thread","fixture-stream");assertNotNull(stream.accept(frames[0].getJSONObject("event")))
    }
    @Test fun removingNestedArrayObjectFieldsDoesNotRestartTheStream() {
        val stream=SnapshotStream();stream.reset("thread","subscription")
        stream.accept(JSONObject("""{"type":"chat.state","data":{"id":"thread","streamID":"subscription","streamRevision":1,"snapshot":{"turns":[{"items":[{"text":"pending","command":"transient","deferred":true}]}]}}}"""))
        val before=stream.accept(JSONObject("""{"type":"chat.delta","data":{"id":"thread","streamID":"subscription","base":1,"streamRevision":2,"ops":[{"op":"remove","path":["snapshot","turns","0","items","0","command"]},{"op":"remove","path":["snapshot","turns","0","items","0","deferred"]},{"op":"set","path":["snapshot","turns","0","items","0","text"],"value":"complete"}]}}"""))!!
        val item=before.getJSONObject("data").getJSONObject("snapshot").getJSONArray("turns").getJSONObject(0).getJSONArray("items").getJSONObject(0)
        assertFalse(item.has("command"));assertFalse(item.has("deferred"));assertEquals("complete",item.getString("text"))
        assertNotNull(stream.accept(JSONObject("""{"type":"chat.delta","data":{"id":"thread","streamID":"subscription","base":2,"streamRevision":3,"ops":[]}}""")))
        assertThrows(IllegalArgumentException::class.java){stream.accept(JSONObject("""{"type":"chat.delta","data":{"id":"thread","streamID":"subscription","base":3,"streamRevision":4,"ops":[{"op":"remove","path":["snapshot","turns","0"]}]}}"""))}
    }
    @Test fun deferredEmptyBodiesAreNotDiscarded() {
        val snapshot=JSONObject("""{"turns":[{"turnId":"t","items":[{"id":"tool","type":"commandExecution","text":"","deferred":true,"bodyRevision":"r1"}]}]}""")
        val messages=conversationMessages(snapshot);assertEquals(1,messages.size);assertTrue(messages[0].deferred);assertEquals("r1",messages[0].bodyRevision)
    }
}
