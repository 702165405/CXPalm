package dev.cxpalm
import org.junit.Assert.assertEquals
import org.junit.Test
class MessageQueueTest {
 @Test fun busyAndPendingQueueUseFIFOWhileIdleStartsOneTurn() {
  assertEquals("chat.send",messageSendMethod(false,false))
  assertEquals("chat.queue",messageSendMethod(true,false))
  assertEquals("chat.queue",messageSendMethod(false,true))
  assertEquals("chat.queue",messageSendMethod(true,true))
 }
}
