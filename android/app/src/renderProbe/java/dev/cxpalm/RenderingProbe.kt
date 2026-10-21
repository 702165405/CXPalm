package dev.cxpalm

import android.app.Activity
import android.app.Instrumentation
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.LazyColumn
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
        val case = intent.getStringExtra("case") ?: "long-code"
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
    override fun onCreate(arguments: Bundle?) {
        super.onCreate(arguments)
        case = arguments?.getString("case") ?: case
        start()
    }
    override fun onStart() {
        val activity = startActivitySync(Intent(targetContext, RenderingProbeActivity::class.java)
            .putExtra("case", case).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        // Markdown parsing and the following Compose measure happen asynchronously.
        Thread.sleep(6000)
        waitForIdleSync()
        runOnMainSync { activity.finish() }
        finish(Activity.RESULT_OK, Bundle().apply { putString("result", "PASS: $case rendered without process death; heap limit=${java.lang.Runtime.getRuntime().maxMemory()}") })
    }
}
