package dev.cxpalm

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import org.json.JSONObject

@Composable internal fun PendingNotice(operation: JSONObject, connected: Boolean, client: BridgeClient) {
    val id=operation.getString("id")
    val status=operation.optString("status")
    val terminal=status in listOf("rejected","not_sent")
    var acknowledge by remember(id) { mutableStateOf(false) }
    Surface(color=MaterialTheme.colorScheme.secondaryContainer,shape=MaterialTheme.shapes.medium) {
        Column(Modifier.fillMaxWidth().heightIn(max=180.dp).verticalScroll(rememberScrollState()).padding(12.dp),verticalArrangement=Arrangement.spacedBy(6.dp)) {
            Row {
                Text(tr(when(status) {"rejected" -> "操作未执行"; "not_sent" -> "操作未发送，草稿已保留"; "unknown" -> "操作结果未知，不会自动重发"; else -> "操作等待确认"}),Modifier.weight(1f),style=MaterialTheme.typography.titleSmall)
                if(terminal) TextButton(onClick={client.dismissOperation(id)}) {Text(tr("关闭提示"))}
            }
            Text("${operation.optString("method")} · $id",style=MaterialTheme.typography.bodySmall)
            if(operation.optBoolean("legacy")) Text(tr("旧版本未记录所属会话。请先在桌面核对这项操作；核对前保留写入保护。"),style=MaterialTheme.typography.bodySmall)
            operation.optString("error").takeIf { it.isNotBlank() }?.let { Text(it,style=MaterialTheme.typography.bodySmall) }
            Row {
                if(!terminal) TextButton(onClick={client.checkPending(id)},enabled=connected) {Text(tr("核对回执"))}
                if(operation.optString("status")=="unknown") TextButton(onClick={acknowledge=true},enabled=connected) {Text(tr("我已检查结果"))}
            }
        }
    }
    if(acknowledge) AlertDialog(onDismissRequest={acknowledge=false},title={Text(tr("确认已人工检查？"))},
        text={Text(tr("先在官方桌面检查实际结果。这只解除这项操作的提示，不会取消、撤销、确认成功或重新发送原操作。"))},
        confirmButton={TextButton(onClick={client.acknowledgePending(id);acknowledge=false}) {Text(tr("已检查，解除提示"))}},
        dismissButton={TextButton(onClick={acknowledge=false}) {Text(tr("返回"))}})
}
