package server

import (
	"context"
	"strings"
	"testing"
)

func TestLegacySuggestionRequestNeverNeedsABridgeOrExecutor(t *testing.T) {
	// A stale client preference must not revive the removed model-generation path.
	s := &Server{}
	value, err := s.execute(context.Background(), "fixture", Command{Method: "chat.suggest", Thread: "fixture", Turn: "fixture"})
	if value != nil || err == nil || !strings.Contains(err.Error(), "independent suggestions removed") {
		t.Fatalf("legacy prediction request was not retired: %v %v", value, err)
	}
}
