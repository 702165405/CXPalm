package desktop

import (
	"regexp"
	"strings"
	"unicode"
)

var displaySecret = regexp.MustCompile(`(?i)(?:\b(?:authorization|api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret)\b["']?\s*[:=]\s*["']?)(?:bearer\s+)?[^\s"',;<>]+|\bbearer\s+[a-z0-9._~+/=-]+|\bsk-[a-z0-9_-]{12,}`)
var displayURL = regexp.MustCompile(`https?://[^\s<>"']+`)

// Forward only display messages, never raw error objects, stacks or response bodies.
func displayError(value any) string {
	message, _ := value.(string)
	if fields, ok := value.(map[string]any); ok {
		message, _ = fields["message"].(string)
	}
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, message)
	message = displaySecret.ReplaceAllString(message, "[REDACTED]")
	// Error URLs may contain credentials, request tokens or private endpoints.
	message = displayURL.ReplaceAllString(message, "[URL]")
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > 4000 {
		return string(runes[:4000]) + "\n[错误信息过长，已截断]"
	}
	return string(runes)
}

func turnDisplayError(turn map[string]any) string {
	if message := displayError(turn["error"]); message != "" {
		return message
	}
	switch turn["status"] {
	case "failed":
		return "本轮执行失败，官方桌面未提供详细原因。"
	case "interrupted":
		return "本轮已中断，官方桌面未提供详细原因。"
	}
	return ""
}
