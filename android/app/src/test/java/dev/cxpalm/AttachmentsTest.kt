package dev.cxpalm

import org.junit.Assert.*
import org.junit.Test

class AttachmentsTest {
    @Test fun uploadCancellationCancelsOnlyItsActualCallIncludingBeforeBinding() {
        val computer=Computer("host","Host","https://host.test","fixture")
        val http=okhttp3.OkHttpClient()
        val first=http.newCall(okhttp3.Request.Builder().url(computer.origin).build())
        val unrelated=http.newCall(okhttp3.Request.Builder().url(computer.origin).build())
        val ticket=AttachmentUploadTicket("upload",readingScope(computer),"chat")
        assertTrue(ticket.matches(computer,"chat"));assertFalse(ticket.matches(computer,"other"))
        assertFalse(ticket.matches(computer.copy(token="replacement"),"chat"))
        assertFalse(ticket.matches(computer.copy(origin="https://other.test"),"chat"))
        ticket.bind(first);ticket.cancel()
        assertTrue(first.isCanceled());assertFalse(unrelated.isCanceled())
        assertFalse(ticket.matches(computer,"chat"))
        val early=AttachmentUploadTicket("early",readingScope(computer),"chat")
        early.cancel();early.bind(unrelated);assertTrue(unrelated.isCanceled())
    }

    @Test fun uploadProxyRejectionIsNotReportedAsStorageQuota() {
        assertTrue(attachmentUploadFailure(413).contains("413"))
        assertTrue(attachmentUploadFailure(413).contains("服务器"))
        assertFalse(attachmentUploadFailure(413).contains("存储配额"))
    }

    @Test fun imageOnlyMessagesAndDecodeBounds() {
        val snapshot=org.json.JSONObject("""{"turns":[{"turnId":"turn","items":[{"id":"generated","type":"imageGeneration","images":[{"id":"${"f".repeat(64)}","name":"result.png"}]}]}]}""")
        val message=conversationMessages(snapshot).single()
        assertEquals("Codex",message.role)
        assertEquals(1,message.images.size)
        assertEquals(4,imageSampleSize(4096,2048,1024))
        assertThrows(IllegalArgumentException::class.java) {imageSampleSize(100000,100000,1024)}
    }
    @Test fun attachmentSizeBounds() {
        assertFalse(validAttachmentSize(0))
        assertFalse(validAttachmentSize(-1))
        assertTrue(validAttachmentSize(1))
        assertTrue(validAttachmentSize(MAX_ATTACHMENT_SIZE))
        assertFalse(validAttachmentSize(MAX_ATTACHMENT_SIZE+1))
    }
}
