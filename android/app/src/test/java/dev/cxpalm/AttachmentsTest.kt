package dev.cxpalm

import org.junit.Assert.*
import org.junit.Test

class AttachmentsTest {
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
