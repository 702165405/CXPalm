package dev.cxpalm

import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Test

class ProtocolTest {
    @Test fun onlyMatchingTerminalReceiptsReleasePendingSend() {
        val receipt=org.json.JSONObject().put("id","original")
        for(status in listOf("confirmed","acknowledged","not_sent","rejected")) {
            receipt.put("status",status)
            org.junit.Assert.assertTrue(settlesPending("original",receipt))
            org.junit.Assert.assertFalse(settlesPending("newer",receipt))
            org.junit.Assert.assertFalse(settlesPending(null,receipt))
        }
        for(status in listOf("dispatched","unknown","","unsupported")) {
            receipt.put("status",status)
            org.junit.Assert.assertFalse(settlesPending("original",receipt))
        }
    }
    @Test fun syncNoticeAppearsOnlyForAnActualConnectedError() {
        val healthy=org.json.JSONObject("""{"total":19,"checked":19,"live":13,"scanning":false}""")
        org.junit.Assert.assertNull(sessionSyncIssue(healthy,true))
        healthy.put("scanning",true);org.junit.Assert.assertNull(sessionSyncIssue(healthy,true))
        healthy.put("error"," list failed ");assertEquals("list failed",sessionSyncIssue(healthy,true))
        org.junit.Assert.assertNull(sessionSyncIssue(healthy,false))
        org.junit.Assert.assertNull(sessionSyncIssue(null,true))
    }

    @Test fun sharedReceiptRequiresVisibleCurrentForegroundReply() {
        val snapshot=org.json.JSONObject("""{"hasUnreadTurn":true,"threadRuntimeStatus":{"type":"idle"}}""")
        val state=ViewState(thread="a",connected=true,desktopConnected=true,sharedReadSync=true,readToken="a".repeat(64),snapshot=snapshot)
        org.junit.Assert.assertTrue(readReceiptEligible(state,true,true))
        snapshot.put("readSyncAvailable",false)
        org.junit.Assert.assertFalse(readReceiptEligible(state,true,true))
        snapshot.put("readSyncAvailable",true)
        org.junit.Assert.assertTrue(readReceiptEligible(state,true,true))
        for(candidate in listOf(state.copy(sharedReadSync=false),state.copy(connected=false),state.copy(desktopConnected=false),state.copy(readToken=""),state.copy(historyPage=org.json.JSONObject()))) {
            org.junit.Assert.assertFalse(readReceiptEligible(candidate,true,true))
        }
        org.junit.Assert.assertTrue(readReceiptEligible(state.copy(historyPage=org.json.JSONObject("""{"hasMore":false}""")),true,true))
        org.junit.Assert.assertFalse(readReceiptEligible(state.copy(historyPage=org.json.JSONObject("""{"hasMore":true}""")),true,true))
        org.junit.Assert.assertFalse(readReceiptEligible(state,false,true))
        org.junit.Assert.assertFalse(readReceiptEligible(state,true,false))
        snapshot.getJSONObject("threadRuntimeStatus").put("type","active")
        org.junit.Assert.assertTrue(readReceiptEligible(state,true,true))
        val data=org.json.JSONObject().put("id","a").put("connected",true).put("sequence",1).put("readToken","b".repeat(64)).put("snapshot",snapshot)
        assertEquals("b".repeat(64),synchronizedChatState(state,data).readToken)
    }

    @Test fun originAcceptsOnlySecureRoot() {
        assertEquals("https://cx.example", normalizeOrigin("https://cx.example/"))
        for (input in listOf("http://cx.example", "https://user:pass@cx.example", "https://cx.example/path", "https://cx.example?token=x")) {
            try { normalizeOrigin(input); fail("Accepted $input") } catch (_: IllegalArgumentException) { }
        }
    }
    @org.junit.Test fun missingItemIDsStillHaveStableUniqueKeys() {
        val snapshot=org.json.JSONObject("""{"turns":[{"items":[{"type":"agentMessage","text":"a"},{"type":"agentMessage","text":"b"}]},{"items":[{"type":"agentMessage","text":"c"}]}]}""")
        val keys=conversationMessages(snapshot).map { it.id }
        org.junit.Assert.assertEquals(3, keys.toSet().size)
        org.junit.Assert.assertEquals(keys, conversationMessages(snapshot).map { it.id })
    }
    @Test fun liveStateIsOrderedAndOfflineTextDoesNotEnableWrites() {
        val snapshot=org.json.JSONObject("""{"threadRuntimeStatus":{"type":"active"}}""")
        val live=ViewState(thread="a",connected=true,desktopConnected=true,snapshot=snapshot,sequence=9)
        val stale=org.json.JSONObject("""{"id":"a","sequence":8,"connected":false}""")
        org.junit.Assert.assertSame(live,synchronizedChatState(live,stale))
        val offline=synchronizedChatState(live,org.json.JSONObject("""{"id":"a","sequence":10,"connected":false}"""))
        org.junit.Assert.assertSame(snapshot,offline.snapshot)
        org.junit.Assert.assertFalse(offline.desktopConnected)
        org.junit.Assert.assertSame(offline,synchronizedChatState(offline,org.json.JSONObject("""{"id":"b","sequence":99,"connected":true}""")))
    }
    @Test fun fullHistoricalPageKeepsLongTextAndStableQuestionTarget() {
        val text="完整内容".repeat(6000)
        val snapshot=org.json.JSONObject().put("turns",org.json.JSONArray().put(org.json.JSONObject().put("turnId","early-turn").put("items",org.json.JSONArray()
            .put(org.json.JSONObject().put("id","q").put("type","userMessage").put("text","first question"))
            .put(org.json.JSONObject().put("id","a").put("type","agentMessage").put("text",text)))))
        val messages=conversationMessages(snapshot)
        assertEquals(2,messages.size)
        assertEquals("early-turn:q",messages[0].id)
        assertEquals(text,messages[1].text)
        org.junit.Assert.assertFalse(messages[1].truncated)
    }

    @Test fun liveEventsDoNotDisplaceHistoricalReadingWindow() {
        val historical=org.json.JSONObject("""{"offset":0,"turns":[{"turnId":"early","items":[]}]}""")
        val state=ViewState(thread="chat",connected=true,sequence=1,historyPage=historical)
        val next=synchronizedChatState(state,org.json.JSONObject("""{"id":"chat","sequence":2,"connected":true,"snapshot":{"turns":[{"turnId":"new","items":[]}]}}"""))
        org.junit.Assert.assertSame(historical,next.historyPage)
        assertEquals("new",next.snapshot?.getJSONArray("turns")?.getJSONObject(0)?.getString("turnId"))
    }

}
