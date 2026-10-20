package dev.cxpalm

import android.content.ContentResolver
import android.net.Uri
import android.provider.OpenableColumns
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okio.BufferedSink
import org.json.JSONObject
import java.io.IOException
import java.util.concurrent.TimeUnit

const val MAX_ATTACHMENT_SIZE = 20L * 1024 * 1024

fun validAttachmentSize(size: Long): Boolean = size in 1..MAX_ATTACHMENT_SIZE

class AttachmentUploadException(val status: Int): IOException("Upload rejected: $status")
fun attachmentUploadFailure(status: Int): String = if(status==413)
    "服务器拒绝了上传请求（HTTP 413）。请求上限为 20M（含表单头），接近上限时请缩小文件后重试。"
else "附件上传失败，请检查连接、文件权限及 20 MiB 大小限制。"

fun uploadAttachment(http: OkHttpClient, resolver: ContentResolver, computer: Computer, thread: String, uri: Uri): JSONObject {
    require(uri.scheme=="content") {"Select a local content file"}
    var name="clipboard.png"
    resolver.query(uri,arrayOf(OpenableColumns.DISPLAY_NAME,OpenableColumns.SIZE),null,null,null)?.use {cursor ->
        if(cursor.moveToFirst()) {
            cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME).takeIf {it>=0}?.let {name=cursor.getString(it)?:name}
            cursor.getColumnIndex(OpenableColumns.SIZE).takeIf {it>=0&&!cursor.isNull(it)}?.let {require(validAttachmentSize(cursor.getLong(it))) {"File size rejected"}}
        }
    }
    val body=object:RequestBody() {
        override fun contentType()="application/octet-stream".toMediaType()
        override fun writeTo(sink: BufferedSink) {
            resolver.openInputStream(uri)?.use {input ->
                val buffer=ByteArray(32768);var total=0L
                while(true){val n=input.read(buffer);if(n<0)break;total+=n;if(total>MAX_ATTACHMENT_SIZE)throw IOException("File too large");sink.write(buffer,0,n)}
                if(total==0L)throw IOException("Empty file")
            } ?: throw IOException("File unavailable")
        }
    }
    val url=computer.origin.toHttpUrlForUpload(thread)
    val request=Request.Builder().url(url).header("Origin",computer.origin).header("Authorization","Bearer ${computer.token}")
        .post(MultipartBody.Builder().setType(MultipartBody.FORM).addFormDataPart("file",name,body).build()).build()
    http.newBuilder().callTimeout(65,TimeUnit.SECONDS).readTimeout(65,TimeUnit.SECONDS).build().newCall(request).execute().use {response ->
        if(response.code!=201)throw AttachmentUploadException(response.code)
        val bytes=response.body?.byteStream()?.use {input ->
            val out=java.io.ByteArrayOutputStream();val buffer=ByteArray(1024)
            while(out.size()<=16384){val n=input.read(buffer,0,minOf(buffer.size,16385-out.size()));if(n<0)break;out.write(buffer,0,n)}
            out.toByteArray()
        }?:throw IOException("Missing upload response")
        require(bytes.size<=16384)
        val file=JSONObject(bytes.toString(Charsets.UTF_8))
        require(file.getString("id").matches(Regex("[a-f0-9]{64}")) && validAttachmentSize(file.getLong("size")))
        return file
    }
}

private fun String.toHttpUrlForUpload(thread: String): String =
    this+"/api/v1/attachments?thread="+java.net.URLEncoder.encode(thread,"UTF-8")
