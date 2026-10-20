package dev.cxpalm

import org.junit.Assert.*
import org.junit.Test
import org.json.JSONObject
import java.io.File

class LanguageTest {
    private val values: Map<String, String> by lazy {
        val data = JSONObject(File("../../locales/en.json").readText(Charsets.UTF_8))
        data.keys().asSequence().associateWith { data.getString(it) }
    }
    @Test fun choicesAndDefaultAreIndependentFromLogin() {
        assertEquals("en", languageCode("en", "zh"))
        assertEquals("zh-CN", languageCode("zh-CN", "en"))
        assertEquals("zh-CN", languageCode(null, "zh-TW"))
        assertEquals("en", languageCode(null, "de"))
        assertEquals("en", languageCode("not-a-language", "en"))
    }
    @Test fun cataloguePreservesPlaceholdersAndTranslatesBothDirections() {
        val catalog = LanguageCatalog(values)
        assertEquals("Settings", catalog.text("设置", "en"))
        assertEquals("设置", catalog.text("Settings", "zh-CN"))
        assertEquals("Connecting to: https://example.test\nVerify that this is a computer address you trust.", catalog.text("即将连接：{0}\n请确认这是你信任的电脑地址。", "en", "https://example.test"))
        val slots = Regex("\\{(\\d+)\\}")
        values.forEach { (zh, en) ->
            assertTrue(en.isNotBlank()); assertFalse(Regex("[\\p{IsHan}]").containsMatchIn(en))
            assertEquals(slots.findAll(zh).map { it.value }.sorted().toList(), slots.findAll(en).map { it.value }.sorted().toList())
        }
    }
    @Test fun runningAndCompletedTimingRenderValuesInBothLanguages() {
        val catalog=LanguageCatalog(values)
        assertEquals("用时 12 分 44 秒",catalog.text("用时 {0}","zh-CN",durationText(764351,false)))
        assertEquals("Time taken 12m 44s",catalog.text("用时 {0}","en",durationText(764351,true)))
        val clock=runningClock(TurnTiming("inProgress",1000,null),66000,true)!!
        assertEquals("执行中 · 1:05",catalog.text("执行中 · {0}","zh-CN",clock))
        assertEquals("Running · 1:05",catalog.text("执行中 · {0}","en",clock))
        val source=File("src/main/java/dev/cxpalm/TaskTiming.kt").readText()
        assertFalse(source.contains("%s"))
        assertTrue(source.contains("tr(\"用时 {0}\""));assertTrue(source.contains("tr(\"执行中 · {0}\""))
    }
    @Test fun userContentAndApprovalFieldsAreNeverTranslated() {
        val raw = "中文 code: <script>hello</script>"
        assertEquals(raw, LanguageCatalog(values).text(raw, "en"))
        val request = JSONObject().put("params", JSONObject().put("reason", "设置"))
        assertEquals("设置", approvalSummary(request, "Missing request", "Empty request"))
        assertEquals("Missing request", approvalSummary(JSONObject(), "Missing request", "Empty request"))
    }
}
