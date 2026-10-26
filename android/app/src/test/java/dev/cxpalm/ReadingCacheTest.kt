package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class ReadingCacheTest {
    private val computer=Computer("one","one","https://one.example","secret-one")
    private fun live()=ViewState(current=computer,thread="t",connected=true,desktopConnected=true,readToken="secret-read",snapshot=JSONObject("""{"turns":[{"turnId":"t","items":[{"id":"a","type":"agentMessage","text":"saved reply"}]}],"requests":[{"id":"approval"}],"hasUnreadTurn":true,"threadRuntimeStatus":{"type":"active"},"latestCollaborationMode":{"mode":"plan"}}"""),sessions=listOf(JSONObject("""{"id":"t","status":"running","live":true,"unread":true,"readToken":"secret-read"}""")))
    @Test fun liveTailNeverPrependsUnrequestedEarlierTurnsIntoALateQuestionJump() {
        val page=JSONObject("""{"offset":9,"total":10,"hasMore":false,"turns":[{"turnId":"t9"}]}""")
        val snapshot=JSONObject("""{"historyTotal":10,"turns":[{"turnId":"t8"},{"turnId":"t9"}]}""")
        assertEquals(listOf("t9"),objects(historyDisplay(page,snapshot)!!.optJSONArray("turns")).map { it.optString("turnId") })
    }
    @Test fun cacheKeepsReadingButNeverRestoresExecutionApprovalOrReadAuthority() {
        val state=live();val memory=ReadingMemory();memory.remember(state)
        val restored=memory.get(state.copy(snapshot=null,connected=false,desktopConnected=false,readToken=""))
        assertTrue(restored.cached);assertFalse(restored.desktopConnected);assertEquals("",restored.readToken)
        assertEquals("saved reply",conversationMessages(restored.snapshot!!).single().text)
        assertEquals("plan",sessionControlValues(restored.snapshot)["mode"])
        assertFalse(restored.snapshot.has("requests"));assertFalse(restored.snapshot.has("hasUnreadTurn"))
        assertEquals("unknown",restored.snapshot.getJSONObject("threadRuntimeStatus").getString("type"))
        assertFalse(restored.sessions[0].getBoolean("live"));assertTrue(restored.sessions[0].isNull("unread"));assertFalse(restored.sessions[0].has("readToken"))
        assertFalse(readReceiptEligible(restored.copy(connected=true,desktopConnected=true,sharedReadSync=true,readToken="a".repeat(64)),true,true))
    }
    @Test fun memoryScopeIncludesHostAndAuthorizationAndLiveDataWinsDiskReplay() {
        val state=live();val memory=ReadingMemory();memory.remember(state)
        for(target in listOf(computer.copy(id="other"),computer.copy(origin="https://other.example"),computer.copy(token="changed"))) {
            assertNotEquals(readingScope(computer),readingScope(target));assertNull(memory.get(state.copy(current=target,snapshot=null)).snapshot)
        }
        val scope=readingScope(computer)
        val payload=JSONObject().put("schema",1).put("scope",scope).put("at",System.currentTimeMillis()).put("chats",org.json.JSONArray().put(readingPayload(state).put("thread","t")))
        val fresh=state.copy(snapshot=JSONObject("""{"turns":[{"turnId":"t","items":[{"id":"a","type":"agentMessage","text":"fresh"}]}]}"""))
        memory.remember(fresh);memory.import(scope,payload,state)
        assertEquals("fresh",conversationMessages(memory.get(state.copy(snapshot=null)).snapshot!!).single().text)
        memory.forget(scope);assertNull(memory.get(state.copy(snapshot=null)).snapshot)
    }
    @Test fun staleOrWrongScopeDiskRecordsDoNotReappearAndHistorySurvivesRoundtrip() {
        val state=live().copy(historyPage=JSONObject("""{"offset":0,"hasMore":true,"turns":[],"questions":[]}"""))
        val scope=readingScope(computer);val memory=ReadingMemory()
        val value=JSONObject().put("schema",1).put("scope",scope).put("at",System.currentTimeMillis()).put("chats",org.json.JSONArray().put(readingPayload(state).put("thread","t")))
        memory.import(scope,value,state);assertNotNull(memory.get(state.copy(snapshot=null,historyPage=null)).historyPage)
        memory.forget(scope);value.put("at",System.currentTimeMillis()-700000000);memory.import(scope,value,state);assertNull(memory.get(state.copy(snapshot=null)).snapshot)
    }
}
