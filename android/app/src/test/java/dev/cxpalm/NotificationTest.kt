package dev.cxpalm

import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test

class NotificationTest {
    @Test fun sameEventOnDifferentComputersKeepsItsSource() {
        val data=JSONObject().put("id","a".repeat(64)).put("kind","approval").put("thread","not-selected").put("title","Other chat")
        val a=conversationNotification(data,Computer("a","Work","https://a.example","private"))!!
        val b=conversationNotification(data,Computer("b","Home","https://b.example","private"))!!
        assertEquals("not-selected",a.thread);assertEquals("a",a.computerId);assertEquals("b",b.computerId)
        assertFalse(a.toString().contains("private"))
        data.put("kind","other");assertNull(conversationNotification(data,Computer("a","A","https://a.example","private")))
    }
    @Test fun systemNotificationIdentitySeparatesComputersAndEventsWithoutCredentials() {
        val first = ConversationNotification("a".repeat(64),"complete","thread-one","Title","computer-one","Work")
        val otherComputer = first.copy(computerId="computer-two",computerName="Home")
        val otherEvent = first.copy(id="b".repeat(64),thread="thread-two",kind="approval")
        assertEquals(taskNotificationTag(first),taskNotificationTag(first.copy(title="Updated display text")))
        assertNotEquals(taskNotificationTag(first),taskNotificationTag(otherComputer))
        assertNotEquals(taskNotificationTag(first),taskNotificationTag(otherEvent))
        assertEquals("task:computer-one:"+"a".repeat(64),taskNotificationTag(first))
        assertFalse(taskNotificationTag(first).contains(first.thread))
    }
    @Test fun persistedDedupeIDsAreBoundedAndRejectMalformedData() {
        assertEquals(emptyList<String>(),notificationIDs("not json"))
        val values=(0..299).map { it.toString(16).padStart(64,'0') }
        val seen=notificationIDs(org.json.JSONArray(values+"invalid").toString())
        assertEquals(256,seen.size);assertEquals(values.last(),seen.last())
    }
    @Test fun reconnectCheckpointsReplaceStalePositionsWithoutExpiredStatus() {
        val stale="a".repeat(64); val latest="b".repeat(64)
        assertEquals(latest, notificationResumeCursor(JSONObject().put("cursor",latest).put("resynced",true),stale))
        assertEquals("baseline:account",notificationResumeCursor(JSONObject().put("cursor","baseline:account"),stale))
        assertEquals("",notificationResumeCursor(JSONObject().put("gap",true),stale))
        assertEquals(stale,notificationResumeCursor(JSONObject().put("cursor","x".repeat(129)),stale))
        assertEquals(stale,notificationResumeCursor(JSONObject(),stale))
    }
}
