package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class UiModelsTest {
    @Test fun onlyOfficialUnreadAndActiveWorkNeedSessionIndicators() {
        for(status in listOf("idle","completed","failed","interrupted","unknown")) {
            assertNull(sessionIndicator(status,false));assertNull(sessionIndicator(status,null))
        }
        assertEquals("completed",sessionIndicator("unknown",true))
        assertEquals("completed",sessionIndicator("idle",true))
        assertEquals("failed",sessionIndicator("failed",true))
        assertEquals("interrupted",sessionIndicator("interrupted",true))
        assertEquals("running",sessionIndicator("running",false))
        assertEquals("waiting",sessionIndicator("waiting",false))
        assertNull(sessionIndicator("running",true,true))
    }

    @Test fun continuousHistoryRetainsBothEdgesAndLiveUpdates() {
        fun page(offset: Int,more: Boolean) = JSONObject().put("offset",offset).put("total",60).put("hasMore",more)
            .put("turns",org.json.JSONArray((offset until offset+20).map { JSONObject().put("turnId","t$it") }))
        val window=extendHistory(page(40,false),page(20,true))
        assertEquals(20,window.getInt("offset"));assertFalse(window.getBoolean("hasMore"));assertEquals(40,window.getJSONArray("turns").length())
        val all=extendHistory(window,page(0,true))
        assertEquals(0,all.getInt("offset"));assertEquals(60,all.getJSONArray("turns").length())
        val live=JSONObject("""{"turns":[{"turnId":"t59","status":"completed"},{"turnId":"t60"}]}""")
        val display=historyDisplay(all,live)!!
        assertEquals(61,display.getJSONArray("turns").length());assertEquals("completed",display.getJSONArray("turns").getJSONObject(59).getString("status"))
        assertEquals(20,historyDisplay(page(0,true),live)!!.getJSONArray("turns").length())
    }
    @Test fun sendFollowupOnlyRepositionsOnceTheNewUserMessageArrives() {
        val before=JSONObject("""{"turns":[{"turnId":"t","items":[{"id":"u","type":"userMessage","text":"old"}]}]}""")
        val waiting=ViewState(snapshot=before,sendUserBefore=latestUserKey(before),sendScrollRevision=1)
        assertSame(waiting,sentMessageState(waiting))
        val after=JSONObject("""{"turns":[{"turnId":"new","items":[{"id":"v","type":"userMessage","text":"new"}]}]}""")
        val landed=sentMessageState(waiting.copy(snapshot=after))
        assertNull(landed.sendUserBefore);assertEquals(2L,landed.sendScrollRevision);assertSame(landed,sentMessageState(landed))
    }
    @Test fun fencedCodeAndLongUnicodeMessagesRemainReadable() {
        val blocks=markdownChildren(parseMarkdown("你好\n\n```kotlin\nval x = 1\n```\n结束"))
        assertEquals("你好",markdownInline(blocks.first()).text)
        assertEquals("kotlin",(blocks[1] as org.commonmark.node.FencedCodeBlock).info)
        assertEquals("结束",markdownInline(blocks.last()).text)
        assertEquals(12000,markdownInline(parseMarkdown("长".repeat(12000)).firstChild).text.length)
    }
    @Test fun extremeCodeDimensionsHaveExplicitBoundedPreviews() {
        val longLine = contentPreview("x".repeat(100000))
        assertEquals(1000, longLine.text.length); assertTrue(longLine.truncated)
        val manyLines = contentPreview((1..100000).joinToString("\n"))
        assertEquals(400, manyLines.text.lines().size); assertTrue(manyLines.truncated)
        assertFalse(contentPreview("normal\ncode").truncated)
        assertEquals("\n\nbody\n", contentPreview("\n\nbody\n").text)
    }
    @Test fun conversationProjectionAndApprovalsUseExistingProtocol() {
        val data = JSONObject("""{"turns":[{"turnId":"t","items":[{"id":"1","type":"userMessage","text":"问题"},{"id":"2","type":"agentMessage","text":"回答","truncated":true}]}]}""")
        val messages = conversationMessages(data)
        assertEquals("You", messages.first().role); assertEquals("Codex", messages.last().role); assertTrue(messages.last().truncated)
        assertTrue(approvalSummary(JSONObject("""{"params":{"command":"git status","cwd":"project"}}""")).contains("git status"))
        assertFalse(approvalSummary(JSONObject()).startsWith("{"))
    }
    @Test fun processRowsAreCollapsedByDefaultWithoutLosingCommandOrVisibleSummary() {
        val data = JSONObject("""{"turns":[{"turnId":"one","items":[
            {"id":"tool","type":"commandExecution","command":"git status","aggregatedOutput":"full output","status":"inProgress"},
            {"type":"reasoning","summary":["Visible",{"text":"summary"}],"encrypted_content":"SECRET","rawContent":["SECRET"]},
            {"type":"reasoning","encrypted_content":"SECRET"},
            {"type":"agentMessage","text":"reply"}, {"type":"userMessage","text":"question"}
        ]},{"turnId":"two","items":[{"id":"tool","type":"commandExecution","command":"pwd","aggregatedOutput":""}]}]}""")
        val rows = conversationMessages(data)
        assertEquals(5, rows.size)
        assertEquals("git status", rows[0].command); assertEquals("full output", rows[0].text)
        assertEquals("inProgress", rows[0].status); assertTrue(rows[0].collapsible)
        assertEquals("Visible\nsummary", rows[1].text); assertTrue(rows[1].collapsible)
        assertFalse(rows[2].collapsible); assertFalse(rows[3].collapsible)
        assertEquals("pwd", rows[4].text); assertNotEquals(rows[0].id, rows[4].id)
        assertFalse(rows.any { it.text.contains("SECRET") })
    }
    @Test fun autoReviewIsDistinctFromManualWorkspaceAndFullAccess() {
        val manual=JSONObject("""{"latestThreadSettings":{"activePermissionProfile":{"id":":workspace"},"approvalsReviewer":"user"}}""")
        assertEquals("workspace",sessionControlValues(manual)["permission"])
        manual.getJSONObject("latestThreadSettings").put("approvalsReviewer","auto_review")
        assertEquals("auto-review",sessionControlValues(manual)["permission"])
        assertEquals(mapOf("permission" to "auto-review"),changedControlSettings(mapOf("permission" to "workspace"),sessionControlValues(manual),"permission"))
    }
    @Test fun latestArrowFollowsReadingPositionRatherThanUnreadStatus() {
        assertFalse(showLatestContent(false,false,false))
        assertFalse(showLatestContent(true,false,false))
        assertTrue(showLatestContent(true,false,true))
        assertTrue(showLatestContent(true,true,false))
    }
    @Test fun latestReplyMustReachItsBottomBeforeItIsConsideredViewed() {
        assertFalse(replyBottomReached(-1, 0, null, 600))
        assertFalse(replyBottomReached(3, 1, null, 600))
        assertFalse(replyBottomReached(3, 4, 900, 600))
        assertTrue(replyBottomReached(3, 4, 600, 600))
        assertTrue(replyBottomReached(3, 4, 608, 600))
        assertTrue(replyBottomReached(3, 5, null, 600))
    }
    @Test fun questionNavigatorFollowsLongRepliesAndDoesNotJumpOutOfHistory() {
        val questions = (0..69).map { "t$it" }
        val messages = listOf(Message("q35","You","q",turn="t35"),Message("a35","Codex","long",turn="t35"),
            Message("q36","You","q",turn="t36"),Message("a36","Codex","long",turn="t36"))
        assertEquals(35, activeQuestionIndex(questions,messages,1,false,false))
        assertEquals(36, activeQuestionIndex(questions,messages,3,false,false))
        assertEquals(36, activeQuestionIndex(questions,messages,1,true,false))
        assertEquals(69, activeQuestionIndex(questions,messages,1,true,true))
        assertEquals(-1, activeQuestionIndex(emptyList(),messages,0,true,true))
    }
    @Test fun questionReleaseIgnoresStaleOrInvalidSliderPositions() {
        val rows=listOf(JSONObject("""{"turn":"a"}"""),JSONObject("""{"turn":"b"}"""))
        assertEquals("b",questionAtPosition(rows,1.5f))
        for(position in listOf(-1f,2f,Float.NaN,Float.POSITIVE_INFINITY)) assertNull(questionAtPosition(rows,position))
        assertNull(questionAtPosition(rows.take(1),1f))
        assertNull(questionAtPosition(emptyList(),0f))
        assertNull(questionAtPosition(listOf(JSONObject()),0f))
    }
    @Test fun computerSwitchClearsOtherHostsHistoryAndProtectsUnknownWrites() {
        val first = Computer("one","First","https://first.example","fixture-one")
        val second = Computer("two","Second","https://second.example","fixture-two")
        val current = ViewState(current=first,thread="first-thread",snapshot=JSONObject("""{"id":"first"}"""),
            historyPage=JSONObject(),historyIndex=JSONObject(),historyTarget="old-target",historyLoading=true,
            readToken="first-token",sharedReadSync=true,connected=true,sequence=999,pending="unknown",project="old-project")
        val next = computerConnectionState(current.copy(pending=null),second,"second-thread","second-pending",setOf("second-skill"))
        assertEquals(second,next.current);assertEquals("second-thread",next.thread)
        assertNull(next.snapshot);assertNull(next.historyPage);assertNull(next.historyIndex);assertNull(next.historyTarget)
        assertFalse(next.historyLoading);assertFalse(next.connected);assertFalse(next.sharedReadSync);assertEquals("",next.readToken)
        assertEquals(-1L,next.sequence);assertEquals("",next.project);assertEquals("second-pending",next.pending)
        assertEquals(setOf("second-skill"),next.skills)
    }
    @Test fun hostManagementReturnsThroughSettingsWithoutLeavingTheConversation() {
        assertEquals("settings", appBackDestination("computers"))
        assertEquals("sessions", appBackDestination("settings"))
    }
    @Test fun contextUsageMatchesWebFieldsAndStaysUnknownWithoutAWindow() {
        assertNull(contextUsage(null));assertNull(contextUsage(JSONObject()))
        assertNull(contextUsage(JSONObject("""{"latestTokenUsageInfo":{"last":{"inputTokens":10},"modelContextWindow":0}}""")))
        assertNull(contextUsage(JSONObject("""{"latestTokenUsageInfo":{"last":{"inputTokens":"10"},"modelContextWindow":100}}""")))
        assertEquals(ContextUsage(2500,10000,25),contextUsage(JSONObject("""{"latestTokenUsageInfo":{"last":{"inputTokens":2500},"modelContextWindow":10000}}""")))
        assertEquals(100,contextUsage(JSONObject("""{"latestTokenUsageInfo":{"last":{"inputTokens":13000},"modelContextWindow":10000}}"""))!!.percent)
    }

    @Test fun historyRenderingAndReadFlagsReuseLargeImmutableContent() {
        val turn=JSONObject().put("turnId","large").put("items",org.json.JSONArray().put(JSONObject().put("id","body").put("type","agentMessage").put("text","Large **reply** ".repeat(100000))))
        val turns=org.json.JSONArray().put(turn)
        val snapshot=JSONObject().put("turns",turns).put("historyTotal",1).put("hasUnreadTurn",true)
        val page=JSONObject().put("offset",0).put("total",1).put("hasMore",false).put("turns",turns)
        val displayed=historyDisplay(page,snapshot)!!
        assertSame(turn,displayed.getJSONArray("turns").getJSONObject(0))
        val read=updatedUnread(snapshot,false)!!
        assertSame(turns,read.getJSONArray("turns"));assertTrue(snapshot.getBoolean("hasUnreadTurn"))
        assertSame(read,updatedUnread(read,false))
        assertEquals(1,renderConversationData(page,snapshot).messages.size)
        val start=System.nanoTime();repeat(20){JSONObject(page.toString())};val before=System.nanoTime()-start
        val afterStart=System.nanoTime();repeat(20){historyDisplay(page,snapshot)};val after=System.nanoTime()-afterStart
        println("CXPalm JVM history-copy benchmark: oldMs=${before/1e6}, newMs=${after/1e6}, textChars=1500000, iterations=20")
    }
    @Test fun streamingNeverOverridesDraggingOrFlingingHistory() {
        assertTrue(shouldFollowStream(true,true,false,false))
        assertFalse(shouldFollowStream(true,true,true,false))
        assertFalse(shouldFollowStream(true,true,false,true))
        assertFalse(shouldFollowStream(false,true,false,false))
        assertFalse(shouldFollowStream(true,false,false,false))
    }

    @Test fun expandedRunningToolsDoNotFetchFullOutputOnEveryRevision() {
        val first=Message("tool","commandExecution","",status="inProgress",deferred=true,bodyRevision="v1")
        assertEquals(detailRefreshKey(first),detailRefreshKey(first.copy(bodyRevision="v2")))
        assertEquals("v2",detailRefreshKey(first.copy(status="completed",bodyRevision="v2")))
        assertEquals("v3",detailRefreshKey(first.copy(status="completed",bodyRevision="v3")))
    }
}
