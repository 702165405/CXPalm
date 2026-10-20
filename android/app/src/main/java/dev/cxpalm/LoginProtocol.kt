package dev.cxpalm

import org.json.JSONArray
import org.json.JSONObject
import java.net.URI

fun loginOrigin(input: String): String {
    val uri=URI(input.trim())
    require(uri.rawFragment==null && uri.rawQuery==null && uri.rawUserInfo==null && uri.port in -1..65535 && uri.port!=0) { "请输入可信的 HTTPS 电脑地址，不要包含账号、查询参数或片段。" }
    require(uri.path.isNullOrEmpty() || uri.path in listOf("/","/cxpalm","/cxpalm/")) { "请输入电脑的 HTTPS 地址，不要包含其他路径。" }
    return normalizeOrigin(URI(uri.scheme,uri.rawAuthority,null,null,null).toString())
}
fun loginHTTPError(code: Int): String = when(code) {
    401 -> "账号或密码错误，或电脑尚未设置登录账号。"
    429 -> "登录尝试过多，请稍后再试。"
    403 -> "电脑拒绝登录，请检查 HTTPS 地址和 IP 白名单。"
    404 -> "请先更新电脑上的 CXPalm 网关以使用账号登录。"
    else -> "登录暂不可用，请稍后重试。"
}
fun loginComputer(body: String, origin: String, name: String, remember: Boolean, now: Long=System.currentTimeMillis()/1000): Computer {
    require(body.length<=16384)
    val value=JSONObject(body);val device=value.getJSONObject("device")
    val token=value.getString("token");val id=device.getString("id");val expires=device.getLong("expires")
    require(token.matches(Regex("[0-9a-f]{64}")) && id.matches(Regex("[0-9a-f]{32}")))
    val maximum=if(remember) 30*24*3600L else 24*3600L
    require(expires>now && expires<=now+maximum+60)
    return Computer(id,name.ifBlank { URI(origin).host },normalizeOrigin(origin),token,remember,expires)
}
fun savedComputers(computers: List<Computer>): String = JSONArray(computers.filter { it.remember }.map { c ->
    JSONObject().put("id",c.id).put("name",c.name).put("origin",c.origin).put("token",c.token).put("remember",true).put("expires",c.expires)
}).toString()
