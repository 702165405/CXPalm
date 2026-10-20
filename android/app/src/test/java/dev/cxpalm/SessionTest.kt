package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class SessionTest {
    @Test fun replyDateWinsOverActivityAndMissingDateIsHonest() {
        val reply=1791648000L
        val expected=java.text.SimpleDateFormat("M/dd",java.util.Locale.getDefault()).format(java.util.Date(reply*1000))
        assertEquals(expected,sessionDate(JSONObject().put("lastReplyAt",reply).put("updated",reply+86400)))
        assertEquals(expected,sessionDate(JSONObject().put("updated",reply)))
        assertEquals("—",sessionDate(JSONObject()))
    }
    @Test fun unreadAndReadRemainTriStateFromOfficialMetadata() {
        assertTrue(JSONObject("""{"unread":null}""").isNull("unread"))
        assertTrue(JSONObject("""{"unread":true}""").optBoolean("unread"))
        assertFalse(JSONObject("""{"unread":false}""").optBoolean("unread"))
    }
}
