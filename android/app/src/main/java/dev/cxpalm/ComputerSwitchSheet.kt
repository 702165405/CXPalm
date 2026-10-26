package dev.cxpalm

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import io.github.u702165405.cxpalm.R

internal fun computerConnectionLabel(state: ViewState, computer: Computer): String = if (computer.id == state.current?.id) when {
    state.desktopConnected && state.connected -> tr("官方桌面已连接")
    state.connected -> tr("Agent 已连接")
    state.status.contains("Connecting") -> tr("连接中")
    else -> tr("已断开")
} else when {
    !state.notificationStatus.containsKey(computer.id) -> tr("监听未启动")
    state.notificationStatus[computer.id] == null -> tr("提醒已连接")
    state.notificationStatus[computer.id] == "revoked" -> tr("登录已失效，请重新登录")
    else -> tr("提醒连接中或暂不可用")
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable internal fun ComputerSwitchSheet(state: ViewState, dismiss: () -> Unit, select: (Computer) -> Unit, manage: () -> Unit) {
    ModalBottomSheet(onDismissRequest = dismiss, containerColor = MaterialTheme.colorScheme.surface) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 20.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(tr("切换电脑"), style = MaterialTheme.typography.titleLarge)
            Text(tr("选中标记表示当前控制的电脑，其他电脑的提醒状态单独显示。"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            LazyColumn(Modifier.weight(1f, fill = false), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(state.computers, key = { it.id }) { computer ->
                    val current = computer.id == state.current?.id
                    Surface(onClick = { select(computer) }, modifier = Modifier.fillMaxWidth().semantics { selected = current },
                        shape = MaterialTheme.shapes.large, color = if (current) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainerLow,
                        border = BorderStroke(1.dp, if(current) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant)) {
                        Row(Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            Icon(painterResource(R.drawable.ic_monitor), null, tint = MaterialTheme.colorScheme.primary)
                            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                Text(computer.name, style = MaterialTheme.typography.titleMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Text((if(current) tr("当前电脑")+" · " else "")+computerConnectionLabel(state, computer), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            if(current) Icon(painterResource(R.drawable.ic_check), tr("当前电脑"), tint = MaterialTheme.colorScheme.primary)
                        }
                    }
                }
            }
            OutlinedButton(onClick = manage, modifier = Modifier.fillMaxWidth()) { Text(tr("管理电脑")) }
        }
    }
}
