package dev.cxpalm

import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextDecoration
import org.commonmark.node.*
import org.commonmark.ext.gfm.tables.*
import org.junit.Assert.*
import org.junit.Test

class MarkdownMessageTest {
    @Test fun realMarkdownProducesStyledTextWithoutVisibleDelimiters() {
        val doc = parseMarkdown("**bold** and *italic* and ~~removed~~ and `literal **code**` and [link](https://example.com)")
        val text = markdownInline(doc.firstChild)
        assertEquals("bold and italic and removed and literal **code** and link", text.text)
        assertTrue(text.spanStyles.any { it.item.fontWeight == FontWeight.Bold })
        assertTrue(text.spanStyles.any { it.item.fontStyle == FontStyle.Italic })
        assertTrue(text.spanStyles.any { it.item.fontFamily == FontFamily.Monospace })
        assertTrue(text.spanStyles.any { it.item.textDecoration == TextDecoration.LineThrough })
        assertEquals(1, text.getLinkAnnotations(0, text.length).size)
    }
    @Test fun headingsListsQuotesTablesAndCodeRemainDistinctBlocks() {
        val source = "# Heading\n\n> Quote\n\n1. first\n2. second\n\n- bullet\n\n| A | B |\n| --- | --- |\n| **cell** | value |\n\n```kotlin\nval stars = \"**raw**\"\n```"
        val blocks = markdownChildren(parseMarkdown(source))
        assertTrue(blocks[0] is Heading); assertTrue(blocks[1] is BlockQuote)
        assertTrue(blocks[2] is OrderedList); assertTrue(blocks[3] is BulletList)
        val table = blocks[4] as TableBlock
        val rows = markdownChildren(table).flatMap(::markdownChildren)
        assertEquals(2, rows.size);assertEquals(2, markdownChildren(rows[1]).size)
        assertEquals("cell", markdownInline(rows[1].firstChild).text)
        assertTrue((rows[0].firstChild as TableCell).isHeader)
        assertEquals("kotlin", (blocks[5] as FencedCodeBlock).info)
        assertTrue((blocks[5] as FencedCodeBlock).literal.contains("**raw**"))
    }
    @Test fun linksAreExplicitHttpOnlyAndRemoteImagesNeverBecomeDownloads() {
        for (bad in listOf("javascript:alert(1)","file:///secret","intent://host","data:text/html,test","https://user:pass@example.com","/local/path")) assertNull(safeMarkdownLink(bad))
        assertEquals("https://example.com/path",safeMarkdownLink("https://example.com/path"))
        val text = markdownInline(parseMarkdown("[bad](javascript:alert) ![image](https://example.com/image.png)").firstChild)
        assertEquals("bad [image]",text.text);assertTrue(text.getLinkAnnotations(0,text.length).isEmpty())
        assertTrue(parseMarkdown("<script>alert(1)</script>").firstChild is HtmlBlock)
    }
    @Test fun deeplyNestedInlineContentRetainsItsTextWithoutRecursiveOverflow() {
        val parent = Paragraph()
        var node: Node = parent
        repeat(1000) { val child=StrongEmphasis("**");node.appendChild(child);node=child }
        node.appendChild(Text("still here"))
        assertEquals("still here",markdownInline(parent).text)
    }
    @Test fun streamingIncompleteMarkdownAndLongRepliesDoNotLoseContent() {
        assertTrue(markdownInline(parseMarkdown("**unfinished").firstChild).text.contains("unfinished"))
        val body = "long reply ".repeat(10000)
        assertEquals(body.trimEnd(),markdownInline(parseMarkdown(body).firstChild).text)
        assertEquals("literal * star",markdownInline(parseMarkdown("literal \\* star").firstChild).text)
        assertTrue(parseMarkdown("```\npartial output").firstChild is FencedCodeBlock)
    }
}
