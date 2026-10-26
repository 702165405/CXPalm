package desktop

import (
	"context"
	"cxpalm/internal/config"
	"strings"
	"testing"
)

func TestRenameValidationAndRootedLookup(t *testing.T) {
	for _, name := range []string{"", "a\nb", "a\x00b", strings.Repeat("名", 121), string([]byte{0xff})} {
		if validThreadName(name) {
			t.Fatalf("invalid name accepted: %q", name)
		}
	}
	if !validThreadName("工作 · " + strings.Repeat("名", 110)) {
		t.Fatal("unicode title rejected")
	}
	b := NewBridge(config.Config{CodexHome: t.TempDir()})
	defer b.Close()
	for _, id := range []string{"../outside", "00000000-0000-0000-0000-000000000001"} {
		if _, err := b.Rename(context.Background(), id, "Title"); err == nil {
			t.Fatal("unrooted/missing thread accepted")
		}
	}
}
