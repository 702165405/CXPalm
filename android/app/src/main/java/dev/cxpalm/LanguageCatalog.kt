package dev.cxpalm

class LanguageCatalog(private val messages: Map<String, String>) {
    private val reverse = messages.entries.associate { it.value to it.key }
    private data class Pattern(val regex: Regex, val slots: List<Int>, val key: String)
    private val patterns = messages.flatMap { (zh, en) ->
        if (!Regex("\\{\\d+\\}").containsMatchIn(zh)) emptyList() else listOf(zh, en).map { value ->
            val slots = mutableListOf<Int>()
            var at = 0
            val expression = StringBuilder("^")
            Regex("\\{(\\d+)\\}").findAll(value).forEach { match ->
                expression.append(Regex.escape(value.substring(at, match.range.first))).append("([\\s\\S]*?)")
                slots += match.groupValues[1].toInt(); at = match.range.last + 1
            }
            expression.append(Regex.escape(value.substring(at))).append('$')
            Pattern(Regex(expression.toString()), slots, zh)
        }
    }
    fun text(source: String, language: String, vararg values: Any?): String {
        val key = if (messages.containsKey(source)) source else reverse[source]
        if (key != null) return fill(if (language == "en") messages.getValue(key) else key, values.toList())
        if (values.isNotEmpty()) return fill(source, values.toList())
        patterns.forEach { pattern ->
            val match = pattern.regex.matchEntire(source) ?: return@forEach
            val args = MutableList<String?>(pattern.slots.maxOrNull()!! + 1) { null }
            pattern.slots.forEachIndexed { index, slot -> args[slot] = match.groupValues[index + 1] }
            return fill(if (language == "en") messages.getValue(pattern.key) else pattern.key, args)
        }
        return source
    }
    private fun fill(template: String, values: List<Any?>): String = Regex("\\{(\\d+)\\}").replace(template) { match ->
        values.getOrNull(match.groupValues[1].toInt())?.toString() ?: match.value
    }
}

fun languageCode(saved: String?, system: String): String = when (saved) {
    "zh-CN", "en" -> saved
    else -> if (system.startsWith("zh", ignoreCase = true)) "zh-CN" else "en"
}
