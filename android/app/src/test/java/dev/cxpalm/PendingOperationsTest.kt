package dev.cxpalm

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class PendingOperationsTest {
    @Test fun unknownArchiveIsScopedAndSurvivesReloadAlongsideOtherOperations() {
        val saved="""[{"id":"archive","thread":"a","method":"sessions.archive","status":"unknown","error":"active writer"},{"id":"send","thread":"b","method":"chat.send","status":"dispatched"},{"id":"create","method":"sessions.create","status":"unknown"}]"""
        val operations=pendingOperations(saved,null)
        val state=ViewState(thread="b",pendingOperations=operations)
        assertEquals("archive",state.pendingFor("a")?.getString("id"))
        assertEquals("send",state.pendingFor("b")?.getString("id"))
        assertNull(state.pendingFor("c"))
        assertEquals("create",state.pendingFor(null)?.getString("id"))
        assertTrue(settlesPending("send",JSONObject("""{"id":"send","status":"confirmed"}""")))
        val reloaded=pendingOperations(JSONArray((operations-"send").values.toList()).toString(),null)
        assertEquals(setOf("archive","create"),reloaded.keys)
        assertEquals("active writer",reloaded["archive"]?.getString("error"))
        assertNull(state.copy(pendingOperations=reloaded).pendingFor("b"))
    }
    @Test fun oldIdOnlyJournalNeverGuessesTheConversationOrLosesTheProtection() {
        val operations=pendingOperations(null,"old-operation")
        val state=ViewState(pendingOperations=operations)
        assertTrue(state.pendingFor(null)!!.getBoolean("legacy"))
        assertEquals("old-operation",state.pendingFor("any")?.getString("id"))
        val migrated=pendingOperations(JSONArray(operations.values.toList()).toString(),null)
        assertEquals(setOf("old-operation"),migrated.keys)
        assertNull(state.copy(pendingOperations=migrated-"old-operation").pendingFor("any"))
    }
}
