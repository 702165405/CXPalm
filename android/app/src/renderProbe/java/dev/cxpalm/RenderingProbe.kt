package dev.cxpalm

import android.app.Activity
import android.app.Instrumentation
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import org.json.JSONArray
import org.json.JSONObject

// Opt-in renderer tests, excluded from normal APKs. Fixture clients require an
// empty account store and never connect to an execution owner.
class RenderingProbeActivity : ComponentActivity() {
    private fun fixtureClient(): BridgeClient {
        val secure=SecureStore(this)
        check(secure.get("computers")==null && secure.get("current")==null) { "Use a fresh emulator without saved accounts" }
        return BridgeClient(this)
    }
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        AppLanguage.initialize(this)
        AppAppearance.initialize(this)
        val case = intent.getStringExtra("case") ?: "long-code"
        if(case.startsWith("appearance-")) {
            AppLanguage.select(this,if(case=="appearance-english") "en" else "zh-CN")
            if(case!="appearance-persist") AppAppearance.select(this,if(case=="appearance-dark") "dark" else "light")
            val client=fixtureClient()
            val computers=listOf(Computer("preview-work","工作电脑","https://work.example.test","synthetic-only"),Computer("preview-home","家中电脑","https://home.example.test","synthetic-only"))
            val titles=listOf("优化移动端布局","检查部署状态","整理项目文档","修复导航对齐","完善错误提示")
            val state=ViewState(computers=computers,current=computers[0],connected=true,desktopConnected=true,
                notificationStatus=mapOf(computers[1].id to null),sessions=titles.mapIndexed { i,title -> JSONObject().put("id","preview-$i").put("title",title).put("projectId","preview").put("projectName","产品开发").put("unread",i==0).put("status",if(i==0) "running" else "idle").put("live",true).put("updated",1792800000+i) })
            val updates=UpdateManager(this)
            if(case=="appearance-persist") check(AppAppearance.mode=="dark") {"Appearance did not survive a fresh process"}
            setContent { CxpalmTheme {
                if(case=="appearance-switcher") {
                    var selected by remember { mutableStateOf(state) }
                    Column {
                        Text("fixture-selected:${selected.current?.id}")
                        ComputerSwitchSheet(selected,{}, {selected=selected.copy(current=it,connected=false,desktopConnected=false)}, {})
                    }
                } else PalmApp(state,client,UpdateState(),updates,false,{},{},{},{},{},false,{})
            } }
            return
        }
        if(case in listOf("approval-permission","approval-decline-only","approval-mcp","approval-mcp-session","approval-mcp-identity")) {
            val request=if(case=="approval-permission") JSONObject("""{"id":42,"method":"item/permissions/requestApproval","params":{"reason":"Fixture only: network access","permissions":{"network":{"enabled":true}}}}""")
                else JSONObject("""{"id":43,"method":"item/commandExecution/requestApproval","params":{"command":"fixture only - not executed","availableDecisions":["decline"]}}""")
            if(case.startsWith("approval-mcp")) {
                request.put("method","mcpServer/elicitation/request").put("params",JSONObject("""{"threadId":"synthetic","mode":"form","message":"Fixture only: confirm this tool call","_meta":{"codex_approval_kind":"mcp_tool_call","persist":["session","always"]},"requestedSchema":{"type":"object","properties":{}}}"""))
                if(case.endsWith("identity")) request.getJSONObject("params").put("mode","openai/userVerification")
            }
            setContent { CxpalmTheme { Column {
                var choice by remember { mutableStateOf("none") }
                ApprovalCard(request,true) {choice=it}
                Text("fixture-decision:$choice")
            } } }
            return
        }
        if(case=="sessions-active" || case=="sessions-archived") {
            val client=fixtureClient()
            val archived=case=="sessions-archived"
            val state=ViewState(connected=true,archived=archived,sessions=listOf(JSONObject()
                .put("id","synthetic").put("title","Session preview").put("archived",archived)
                .put("status","idle").put("live",true)))
            setContent { CxpalmTheme { SessionsPage(state,client) } }
            return
        }
        if(case=="composer-preview" || case=="composer-preview-long" || case=="composer-preview-approval" || case=="composer-preview-mcp") {
            val client=fixtureClient()
            client.draft(if(case.endsWith("-long")) "先检查移动端的阅读空间。\n输入多行时再展开。\n保留附件、引导和排队功能。" else "")
            val computer=Computer("preview","本地工作台","https://fixture.invalid","synthetic-only")
            val turns=JSONArray()
            listOf(
                "这个页面最需要优化哪里？" to "优先把空间留给会话正文。输入区只保留最常用的操作，其余功能在需要时展开。",
                "会话通知还需要保留。" to "通知仍按会话单独控制，不改变订阅和授权逻辑。这是一段用于界面预览的示例内容，不是真实模型执行结果。",
                "附件和长消息也要能正常使用。" to "可以继续选择文件、粘贴图片和输入多行消息。发送、引导与排队的行为保持不变。"
            ).forEachIndexed { i,(question,answer) ->
                turns.put(JSONObject().put("turnId","preview-$i").put("status","completed").put("items",JSONArray()
                    .put(JSONObject().put("id","question-$i").put("type","userMessage").put("text",question))
                    .put(JSONObject().put("id","answer-$i").put("type","agentMessage").put("text",answer))))
            }
            val snapshot=JSONObject().put("turns",turns).put("threadRuntimeStatus",JSONObject().put("type","idle"))
                .put("latestModel","gpt-5.3-codex").put("latestReasoningEffort","high")
                .put("latestThreadSettings",JSONObject().put("activePermissionProfile",JSONObject().put("id",":workspace")))
                .put("notificationPreference",JSONObject().put("enabled",false))
                .put("latestTokenUsageInfo",JSONObject().put("last",JSONObject().put("inputTokens",8400)).put("modelContextWindow",200000))
            val state=ViewState(computers=listOf(computer),current=computer,thread="preview-chat",connected=true,desktopConnected=true,
                sessions=listOf(JSONObject().put("id","preview-chat").put("title","界面预览 · 示例会话")),snapshot=snapshot)
            if(case=="composer-preview-approval") snapshot.put("requests",JSONArray("""[{"id":42,"method":"item/permissions/requestApproval","params":{"threadId":"preview-chat","reason":"Fixture network access","permissions":{"network":{"enabled":true}}}}]"""))
            if(case=="composer-preview-mcp") snapshot.put("requests",JSONArray("""[{"id":"mcp-fixture","method":"mcpServer/elicitation/request","params":{"threadId":"preview-chat","mode":"form","message":"Fixture only: confirm this tool call","_meta":{"codex_approval_kind":"mcp_tool_call"},"requestedSchema":{"type":"object","properties":{}}}}]"""))
            val updates=UpdateManager(this)
            setContent { CxpalmTheme { PalmApp(state,client,UpdateState(),updates,false,{},{},{},{},{},false,{}) } }
            return
        }
        if(case.startsWith("archive-")) {
            check(case in listOf("archive-rejected","archive-other-chat","archive-unknown","archive-legacy"))
            val client=fixtureClient()
            val operation=JSONObject().put("id","synthetic-archive").put("thread","synthetic").put("method","sessions.archive")
                .put("status",if(case=="archive-rejected" || case=="archive-other-chat") "rejected" else "unknown")
                .put("error","Official desktop retains write ownership. Archive was not executed. ".repeat(12))
            if(case=="archive-legacy")operation.remove("thread").also {operation.put("legacy",true)}
            val terminal=operation.optString("status")=="rejected"
            val thread=if(case=="archive-other-chat") "another" else "synthetic"
            val state=ViewState(current=Computer("synthetic","synthetic","https://fixture.invalid","synthetic-only"),thread=thread,
                connected=true,desktopConnected=true,snapshot=JSONObject().put("turns",JSONArray()),
                pending=if(!terminal) "synthetic-archive" else null,
                pendingOperations=if(terminal) emptyMap() else mapOf("synthetic-archive" to operation),
                operationFailures=if(terminal) mapOf("synthetic" to operation) else emptyMap())
            setContent { CxpalmTheme { ChatPage(state,client) } }
            return
        }
        if(case.matches(Regex("capture-[0-9]+"))) {
            val root=getExternalFilesDir("captures")!!
            val file=java.io.File(root,"$case.json")
            check(file.canonicalFile.parentFile==root.canonicalFile)
            val captured=JSONObject(file.readText())
            val snapshot=captured.getJSONObject("snapshot")
            val index=seedHistoryIndex(captured.getJSONObject("historyIndex"),snapshot,true)
            val client=fixtureClient()
            val state=ViewState(current=Computer("synthetic","synthetic","https://fixture.invalid","synthetic-only"),thread="synthetic",
                snapshot=snapshot,historyIndex=index,cached=true)
            setContent { CxpalmTheme { ChatPage(state,client) } }
            return
        }
        if(case=="chat" || case=="unmatched-question") {
            val client=fixtureClient()
            val turns=JSONArray();val questions=JSONArray()
            repeat(20) { i ->
                turns.put(JSONObject().put("turnId","turn-$i").put("status","completed").put("items",JSONArray()
                    .put(JSONObject().put("id","q").put("type","userMessage").put("text","question $i"))
                    .put(JSONObject().put("id","a").put("type","agentMessage").put("text","reply $i ".repeat(1000)))))
                questions.put(JSONObject().put("turn",if(case=="unmatched-question") "not-loaded-$i" else "turn-$i").put("text","question $i"))
            }
            val state=ViewState(current=Computer("synthetic","synthetic","https://fixture.invalid","synthetic-only"),thread="synthetic",
                snapshot=JSONObject().put("turns",turns),historyIndex=JSONObject().put("questions",questions),cached=true)
            setContent { CxpalmTheme { ChatPage(state,client) } }
            return
        }
        val source = when (case) {
            "long-code" -> "```text\n" + "x".repeat(200000) + "\n```"
            "many-lines" -> "```text\n" + "output\n".repeat(20000) + "```"
            "paragraph" -> "word ".repeat(50000)
            "table" -> "| A | B |\n| --- | --- |\n" + "| cell | value |\n".repeat(4000)
            "user" -> "user text ".repeat(20000)
            else -> error("Unsupported rendering probe")
        }
        setContent {
            CxpalmTheme {
                LazyColumn(Modifier.fillMaxSize()) {
                    item {
                        MessageCard(Message("synthetic", if(case=="user") "You" else "Codex", source),
                            { error("No network in renderer fixture") }, { _, _ -> error("No network in renderer fixture") })
                    }
                }
            }
        }
    }
}

class RenderingProbeRunner : Instrumentation() {
    private var case = "long-code"
    private var holdMs = 6000L
    override fun onCreate(arguments: Bundle?) {
        super.onCreate(arguments)
        case = arguments?.getString("case") ?: case
        if(case.startsWith("composer-preview") || case=="chat") holdMs=arguments?.getString("holdMs")?.toLongOrNull()?.coerceIn(6000,60000) ?: holdMs
        start()
    }
    override fun onStart() {
        val automation=if(case.startsWith("appearance-") || case=="sessions-active" || case=="sessions-archived" || case.startsWith("approval-") || case=="composer-preview-approval" || case=="composer-preview-mcp") uiAutomation else null
        val activity = startActivitySync(Intent(targetContext, RenderingProbeActivity::class.java)
            .putExtra("case", case).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        // Markdown parsing and the following Compose measure happen asynchronously.
        Thread.sleep(holdMs)
        waitForIdleSync()
        if(case.startsWith("appearance-")) {
            fun nodes(node: android.view.accessibility.AccessibilityNodeInfo): List<android.view.accessibility.AccessibilityNodeInfo> = listOf(node)+(0 until node.childCount).flatMap { node.getChild(it)?.let(::nodes).orEmpty() }
            fun visible()=nodes(checkNotNull(automation?.rootInActiveWindow))
            fun find(text: String): android.view.accessibility.AccessibilityNodeInfo? {
                repeat(15) {
                    visible().firstOrNull {it.text?.toString()==text}?.let {return it}
                    Thread.sleep(200)
                }
                return null
            }
            fun click(text: String) {
                var node=checkNotNull(find(text)) {"Missing label $text"}
                while(!node.isClickable) node=checkNotNull(node.parent)
                check(node.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK))
                Thread.sleep(700);waitForIdleSync()
            }
            if(case=="appearance-switcher") {
                check(find("工作电脑")!=null && find("家中电脑")!=null)
                click("家中电脑")
                check(find("当前电脑 · 已断开")!=null)
            } else {
                check(find("工作电脑")!=null)
                check(find(tr("切换电脑"))!=null)
                check(find("优化移动端布局")!=null)
                if(case=="appearance-light") {
                    click("未读")
                    check(find("仅筛选已加载会话，可继续加载更多。")!=null)
                    check(visible().none {it.text?.toString()=="检查部署状态"})
                    click("全部");check(find("检查部署状态")!=null)
                }
                if(case=="appearance-english") check(find("Computers")!=null)
                if(case=="appearance-settings") {
                    click("设置");check(find("外观")!=null)
                    click("雾蓝夜间");check(AppAppearance.mode=="dark")
                    check(targetContext.getSharedPreferences("cxpalm-ui",android.content.Context.MODE_PRIVATE).getString("appearance",null)=="dark")
                    click("明亮工作台");check(AppAppearance.mode=="light")
                    click("跟随系统");check(AppAppearance.mode=="system")
                    click("雾蓝夜间")
                }
            }
            val screenshot=checkNotNull(automation?.takeScreenshot())
            val folder=activity.getExternalFilesDir("previews")!!;folder.mkdirs()
            java.io.File(folder,"$case.png").outputStream().use {screenshot.compress(android.graphics.Bitmap.CompressFormat.PNG,100,it)}
            screenshot.recycle()
        }
        if(case.startsWith("approval-") || case=="composer-preview-approval" || case=="composer-preview-mcp") {
            fun nodes(node: android.view.accessibility.AccessibilityNodeInfo): List<android.view.accessibility.AccessibilityNodeInfo> =
                listOf(node)+(0 until node.childCount).flatMap { index -> node.getChild(index)?.let(::nodes).orEmpty() }
            fun visible()=nodes(checkNotNull(automation?.rootInActiveWindow))
            fun find(vararg text: String)=visible().firstOrNull {it.text?.toString() in text}
            fun click(node: android.view.accessibility.AccessibilityNodeInfo) {
                var target=node
                while(!target.isClickable) target=checkNotNull(target.parent)
                check(target.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK))
                Thread.sleep(400);waitForIdleSync()
            }
            if(case=="composer-preview-mcp") {
                check(checkNotNull(find("仅批准这一次","Approve once")).isVisibleToUser)
            } else if(case=="approval-mcp-identity") {
                check(find("仅批准这一次","Approve once")==null)
            } else if(case=="approval-mcp-session") {
                click(checkNotNull(find("本会话内始终批准","Always approve in this conversation")))
                check(find("fixture-decision:acceptForSession")==null)
                click(checkNotNull(find("确认批准","Confirm approval")))
                check(find("fixture-decision:acceptForSession")!=null)
            } else if(case=="approval-mcp") {
                click(checkNotNull(find("仅批准这一次","Approve once")))
                check(find("fixture-decision:accept")==null)
                click(checkNotNull(find("确认批准","Confirm approval")))
                check(find("fixture-decision:accept")!=null)
            } else if(case=="approval-decline-only") {
                check(find("仅批准这一次","Approve once")==null)
                click(checkNotNull(find("拒绝","Decline")))
                check(find("fixture-decision:decline")!=null)
            } else {
                val approve=checkNotNull(find("批准本轮权限","Approve for this turn"))
                check(approve.isVisibleToUser)
                if(case=="approval-permission") {
                    click(approve)
                    check(find("fixture-decision:accept")==null) {"Approval skipped confirmation"}
                    click(checkNotNull(find("确认批准","Confirm approval")))
                    check(find("fixture-decision:accept")!=null)
                }
            }
        }
        if(case=="sessions-active" || case=="sessions-archived") {
            fun labels(node: android.view.accessibility.AccessibilityNodeInfo): List<String> =
                listOfNotNull(node.text?.toString(),node.contentDescription?.toString()) +
                    (0 until node.childCount).flatMap { index -> node.getChild(index)?.let(::labels).orEmpty() }
            val visible=labels(checkNotNull(automation?.rootInActiveWindow))
            check(visible.any { it.contains("Session preview") }) { "Session list did not render" }
            check(visible.none { it in listOf("归档","取消归档","Archive","Unarchive") }) { "Unsupported archive action is visible" }
        }
        runOnMainSync { activity.finish() }
        finish(Activity.RESULT_OK, Bundle().apply { putString("result", "PASS: $case rendered without process death; heap limit=${java.lang.Runtime.getRuntime().maxMemory()}") })
    }
}
