package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class TaskTimingTest {
    @Test fun actualTimingVariantsAndMissingValues() {
        assertEquals(12000L,turnTiming(JSONObject("""{"status":"completed","turnDurationMs":12000}""")).duration)
        assertEquals(65000L,turnTiming(JSONObject("""{"turnStartedAtMs":1000,"turnCompletedAtMs":66000}""")).duration)
        assertEquals(0L,turnTiming(JSONObject("""{"durationMs":0}""")).duration)
        assertNull(turnTiming(JSONObject("""{"durationMs":"12000"}""")).duration)
        assertNull(turnTiming(JSONObject("""{"turnStartedAtMs":2000,"turnCompletedAtMs":1000}""")).duration)
        assertNull(turnTiming(JSONObject("""{"durationMs":-1}""")).duration)
    }
    @Test fun runningUsesOfficialStartAndStopsWhenOffline() {
        val timing=TurnTiming("inProgress",1000,null)
        assertEquals("1:05",runningClock(timing,66000,true))
        assertEquals("1:01:01",runningClock(timing,3662000,true))
        assertNull(runningClock(timing,66000,false))
        assertNull(runningClock(timing,500,true))
        assertNull(runningClock(TurnTiming("completed",1000,60000),66000,true))
    }
    @Test fun durationAndPlacementMatchBothLanguagesAndEachTurn() {
        assertEquals("12 分 44 秒",durationText(764351,false))
        assertEquals("12m 44s",durationText(764351,true))
        assertEquals("0 秒",durationText(0,false))
        val messages=listOf(Message("u","You","q",turn="a"),Message("c1","Codex","thinking",turn="a"),Message("c2","Codex","answer",turn="a"),Message("tool","commandExecution","tool",turn="a"),Message("u2","You","q2",turn="b"),Message("tool2","commandExecution","tool2",turn="b"))
        assertEquals(setOf(2,5),timingFooterIndices(messages))
        val parsed=conversationMessages(JSONObject("""{"turns":[{"turnId":"real-turn","items":[{"type":"agentMessage","text":"answer"}]}]}"""))
        assertEquals("real-turn",parsed.single().turn)
    }
}
