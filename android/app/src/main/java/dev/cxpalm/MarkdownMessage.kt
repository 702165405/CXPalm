package dev.cxpalm

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.*
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import org.commonmark.node.*
import org.commonmark.parser.Parser
import org.commonmark.ext.gfm.tables.*
import org.commonmark.ext.gfm.strikethrough.Strikethrough
import org.commonmark.ext.gfm.strikethrough.StrikethroughExtension
import java.net.URI

private val markdownParser = Parser.builder().extensions(listOf(TablesExtension.create(), StrikethroughExtension.create())).build()
internal fun parseMarkdown(source: String): Node = markdownParser.parse(source)
internal fun safeMarkdownLink(value: String): String? = runCatching {
    val uri = URI(value)
    value.takeIf { uri.scheme?.lowercase() in listOf("https", "http") && !uri.host.isNullOrEmpty() && uri.userInfo == null }
}.getOrNull()
internal fun markdownChildren(parent: Node): List<Node> = buildList {
    var child = parent.firstChild
    while (child != null) { add(child); child = child.next }
}
private fun markdownPlain(root: Node): String = buildString {
    val pending = java.util.ArrayDeque<Node>()
    pending.add(root)
    while (pending.isNotEmpty()) {
        val node = pending.removeLast()
        when (node) {
            is org.commonmark.node.Text -> append(node.literal)
            is Code -> append(node.literal)
            is FencedCodeBlock -> append(node.literal)
            is IndentedCodeBlock -> append(node.literal)
            is HtmlInline -> append(node.literal)
            is HtmlBlock -> append(node.literal)
            is SoftLineBreak, is HardLineBreak -> append('\n')
            else -> markdownChildren(node).asReversed().forEach(pending::add)
        }
    }
}
internal fun markdownInline(parent: Node, linkColor: Color = Color.Blue, codeColor: Color = Color.LightGray): AnnotatedString = buildAnnotatedString {
    fun visit(node: Node, depth: Int = 0) {
        if (depth > 64) { append(markdownPlain(node)); return }
        fun children() { markdownChildren(node).forEach { visit(it, depth + 1) } }
        when (node) {
            is org.commonmark.node.Text -> append(node.literal)
            is Code -> withStyle(SpanStyle(fontFamily = FontFamily.Monospace, background = codeColor)) { append(node.literal) }
            is SoftLineBreak -> append(' ')
            is HardLineBreak -> append('\n')
            is HtmlInline -> append(node.literal)
            is Emphasis -> withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { children() }
            is StrongEmphasis -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { children() }
            is Strikethrough -> withStyle(SpanStyle(textDecoration = TextDecoration.LineThrough)) { children() }
            is Link -> {
                val url = safeMarkdownLink(node.destination)
                if (url == null) children()
                else withLink(LinkAnnotation.Url(url, TextLinkStyles(SpanStyle(color = linkColor, textDecoration = TextDecoration.Underline)))) { children() }
            }
            is Image -> { append('['); children(); append(']') }
            else -> children()
        }
    }
    markdownChildren(parent).forEach { visit(it) }
}

@Composable fun MarkdownMessage(source: String) {
    val document by produceState<Node>(Document(),source) {
        value=kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Default) { parseMarkdown(source) }
    }
    SelectionContainer {
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            MarkdownBlocks(markdownChildren(document))
        }
    }
}

@Composable private fun MarkdownText(node: Node, modifier: Modifier = Modifier, style: TextStyle = MaterialTheme.typography.bodyLarge) {
    val colors = MaterialTheme.colorScheme
    val text = remember(node, colors.primary, colors.surfaceVariant) { markdownInline(node, colors.primary, colors.surfaceVariant) }
    Text(text, modifier, style = style)
}

@Composable private fun MarkdownBlocks(nodes: List<Node>, depth: Int = 0) {
    for (node in nodes) {
        if (depth > 32) { MarkdownText(node); continue }
        when (node) {
            is org.commonmark.node.Paragraph -> MarkdownText(node)
            is Heading -> MarkdownText(node, Modifier.semantics { heading() }, when (node.level) {
                1 -> MaterialTheme.typography.headlineSmall
                2 -> MaterialTheme.typography.titleLarge
                else -> MaterialTheme.typography.titleMedium
            })
            is FencedCodeBlock -> CodeText(node.literal.trimEnd('\n'), node.info.substringBefore(' ').take(40))
            is IndentedCodeBlock -> CodeText(node.literal.trimEnd('\n'))
            is HtmlBlock -> Text(node.literal.trimEnd('\n'), style = MaterialTheme.typography.bodyMedium)
            is ThematicBreak -> HorizontalDivider(Modifier.padding(vertical = 4.dp))
            is BlockQuote -> Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Spacer(Modifier.width(3.dp).fillMaxHeight().background(MaterialTheme.colorScheme.outlineVariant))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) { MarkdownBlocks(markdownChildren(node), depth + 1) }
            }
            is BulletList, is OrderedList -> Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                markdownChildren(node).forEachIndexed { index, item ->
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(if (node is OrderedList) "${(node.markerStartNumber ?: 1) + index}." else "\u2022", Modifier.widthIn(min = 20.dp), style = MaterialTheme.typography.bodyLarge)
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) { MarkdownBlocks(markdownChildren(item), depth + 1) }
                    }
                }
            }
            is TableBlock -> MarkdownTable(node)
            else -> MarkdownBlocks(markdownChildren(node), depth + 1)
        }
    }
}

@Composable private fun MarkdownTable(table: TableBlock) {
    val rows = markdownChildren(table).flatMap(::markdownChildren)
    Surface(shape = MaterialTheme.shapes.small, border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant)) {
        Column(Modifier.horizontalScroll(rememberScrollState())) {
            rows.forEachIndexed { index, row ->
                if (index > 0) HorizontalDivider()
                Row(Modifier.height(IntrinsicSize.Min)) {
                    markdownChildren(row).forEachIndexed { column, cell ->
                        if (column > 0) VerticalDivider()
                        MarkdownText(cell, Modifier.width(168.dp).padding(10.dp),
                            if ((cell as? TableCell)?.isHeader == true) MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.Bold) else MaterialTheme.typography.bodyMedium)
                    }
                }
            }
        }
    }
}
