package dev.cxpalm

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp

@Composable fun LoginDialog(client: BridgeClient, initialAddress: String, dismiss: () -> Unit) {
    val state by client.state.collectAsState()
    LaunchedEffect(Unit){client.clearError()}
    var address by remember { mutableStateOf(initialAddress) }
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var rememberLogin by remember { mutableStateOf(false) }
    val valid=runCatching {loginOrigin(address)}.isSuccess && username.isNotBlank() && password.isNotEmpty()
    AlertDialog(onDismissRequest={if(!client.loginBusy){password="";dismiss()}},title={Text(tr("账号登录"))},text={
        Column(Modifier.verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(12.dp)) {
            state.error?.let {Text(tr(it),color=MaterialTheme.colorScheme.error,style=MaterialTheme.typography.bodySmall)}
            Text(tr("先在电脑 CXPalm 的「设置 → 登录账号」创建账号密码。"),style=MaterialTheme.typography.bodySmall)
            OutlinedTextField(address,{address=it},label={Text(tr("电脑 HTTPS 地址"))},singleLine=true,keyboardOptions=KeyboardOptions(keyboardType=KeyboardType.Uri),modifier=Modifier.fillMaxWidth(),enabled=!client.loginBusy)
            OutlinedTextField(username,{username=it},label={Text(tr("账号"))},singleLine=true,modifier=Modifier.fillMaxWidth(),enabled=!client.loginBusy)
            OutlinedTextField(password,{password=it},label={Text(tr("密码"))},singleLine=true,visualTransformation=PasswordVisualTransformation(),keyboardOptions=KeyboardOptions(keyboardType=KeyboardType.Password),modifier=Modifier.fillMaxWidth(),enabled=!client.loginBusy)
            Row(verticalAlignment=androidx.compose.ui.Alignment.CenterVertically) {Checkbox(rememberLogin,{rememberLogin=it},enabled=!client.loginBusy);Text(tr("记住登录 · 30 天内无需重新登录"),style=MaterialTheme.typography.bodyMedium)}
            Text(tr("只保存加密的登录凭据，不保存明文密码。不勾选时仅在本次 App 运行期间保留登录。"),style=MaterialTheme.typography.bodySmall)
        }
    },confirmButton={TextButton(onClick={client.clearError();client.login("",address,username,password,rememberLogin){password="";dismiss()};password=""},enabled=valid&&!client.loginBusy){Text(if(client.loginBusy) tr("正在登录…") else tr("登录"))}},dismissButton={TextButton(onClick={password="";dismiss()},enabled=!client.loginBusy){Text(tr("取消"))}})
}
