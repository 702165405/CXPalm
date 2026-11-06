package dev.cxpalm

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.InputStream

data class MessageFile(val id: String, val name: String, val kind: String)
private data class FileSaveRequest(val file: MessageFile, val load: suspend () -> ByteArray)

internal fun returnedFileBytes(input: InputStream): ByteArray {
    val output=java.io.ByteArrayOutputStream();val buffer=ByteArray(32768)
    while(output.size()<=MAX_ATTACHMENT_SIZE) {
        val count=input.read(buffer,0,minOf(buffer.size,MAX_ATTACHMENT_SIZE.toInt()+1-output.size()))
        if(count<0)break
        output.write(buffer,0,count)
    }
    require(output.size()<=MAX_ATTACHMENT_SIZE) {"File exceeds download limit"}
    return output.toByteArray()
}

@Composable fun MessageFiles(message: Message, load: suspend (Message,MessageFile)->ByteArray) {
    val resolver=LocalContext.current.contentResolver
    val scope=rememberCoroutineScope()
    var pending by remember(message.id) {mutableStateOf<FileSaveRequest?>(null)}
    var saving by remember(message.id) {mutableStateOf(false)}
    var status by remember(message.id) {mutableStateOf("")}
    val save=rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/octet-stream")) {uri ->
        val request=pending;pending=null
        if(uri!=null && request!=null) scope.launch {
            saving=true;status="正在保存…"
            try {
                val bytes=request.load()
                withContext(Dispatchers.IO) {resolver.openOutputStream(uri,"wt")?.use {it.write(bytes)}?:error("Save destination unavailable")}
                status="文件已保存"
            }catch(cancelled:kotlinx.coroutines.CancellationException) {throw cancelled}
            catch(_:Exception) {status="保存失败，请检查电脑连接、原文件和保存位置。"}
            finally {saving=false}
        }
    }
    for(file in message.files) Surface(shape=MaterialTheme.shapes.medium,border=BorderStroke(1.dp,MaterialTheme.colorScheme.outlineVariant)) {
        Row(Modifier.fillMaxWidth().padding(12.dp),verticalAlignment=Alignment.CenterVertically,horizontalArrangement=Arrangement.spacedBy(8.dp)) {
            Column(Modifier.weight(1f)) {
                Text(file.name,style=MaterialTheme.typography.bodyMedium)
                Text(file.kind.uppercase(),style=MaterialTheme.typography.labelSmall,color=MaterialTheme.colorScheme.onSurfaceVariant)
            }
            TextButton(enabled=!saving&&pending==null,onClick={
                pending=FileSaveRequest(file) {load(message,file)}
                save.launch(file.name)
            }) {Text(tr("保存文件"))}
        }
    }
    if(status.isNotEmpty()) Text(tr(status),style=MaterialTheme.typography.bodySmall)
}
