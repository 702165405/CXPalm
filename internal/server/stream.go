package server

import (
	"cxpalm/internal/desktop"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type streamOp struct {
	Op    string   `json:"op"`
	Path  []string `json:"path"`
	Value any      `json:"value"`
}
type streamEncoder struct {
	previous     map[string]any
	revision     uint64
	subscription string
}

func (e *streamEncoder) next(state desktop.State, subscription string) Event {
	raw, _ := json.Marshal(state)
	var current map[string]any
	_ = json.Unmarshal(raw, &current)
	if e.subscription != subscription {
		e.previous = nil
		e.revision = 0
		e.subscription = subscription
	}
	e.revision++
	var ops []streamOp
	if e.previous != nil {
		diffStream(nil, e.previous, current, &ops)
	}
	patch := map[string]any{"id": state.ID, "streamID": subscription, "base": e.revision - 1, "streamRevision": e.revision, "ops": ops}
	encoded, _ := json.Marshal(patch)
	full := Event{Type: "chat.state", Data: mapWithStream(current, subscription, e.revision)}
	event := full
	if e.previous != nil && len(encoded) < len(raw)*3/4 {
		event = Event{Type: "chat.delta", Data: patch}
	}
	e.previous = current
	return event
}
func mapWithStream(current map[string]any, id string, revision uint64) map[string]any {
	out := make(map[string]any, len(current)+2)
	for k, v := range current {
		out[k] = v
	}
	out["streamID"] = id
	out["streamRevision"] = revision
	return out
}
func diffStream(path []string, before, after any, ops *[]streamOp) {
	if len(path) >= 20 {
		if !reflect.DeepEqual(before, after) {
			*ops = append(*ops, streamOp{Op: "set", Path: append([]string{}, path...), Value: after})
		}
		return
	}
	child := func(key string) []string { out := append([]string{}, path...); return append(out, key) }
	if a, ok := before.(map[string]any); ok {
		if b, ok := after.(map[string]any); ok {
			for key := range a {
				if _, ok := b[key]; !ok {
					*ops = append(*ops, streamOp{Op: "remove", Path: child(key)})
				}
			}
			for key, value := range b {
				if old, ok := a[key]; ok {
					diffStream(child(key), old, value, ops)
				} else {
					*ops = append(*ops, streamOp{Op: "set", Path: child(key), Value: value})
				}
			}
			return
		}
	}
	if a, ok := before.([]any); ok {
		if b, ok := after.([]any); ok {
			for i, value := range b {
				if i < len(a) {
					diffStream(child(strconv.Itoa(i)), a[i], value, ops)
				} else {
					*ops = append(*ops, streamOp{Op: "set", Path: child(strconv.Itoa(i)), Value: value})
				}
			}
			if len(b) < len(a) {
				*ops = append(*ops, streamOp{Op: "resize", Path: append([]string{}, path...), Value: len(b)})
			}
			return
		}
	}
	if a, ok := before.(string); ok {
		if b, ok := after.(string); ok && len(b) > len(a) && len(a) > 32 && strings.HasPrefix(b, a) {
			*ops = append(*ops, streamOp{Op: "append", Path: append([]string{}, path...), Value: b[len(a):]})
			return
		}
	}
	if !reflect.DeepEqual(before, after) {
		*ops = append(*ops, streamOp{Op: "set", Path: append([]string{}, path...), Value: after})
	}
}

type streamDelivery struct {
	state        desktop.State
	subscription string
	stream       bool
}

// One replaceable presentation slot. Receipts/approvals/notifications keep their reliable queue.
type latestState struct {
	mu           sync.Mutex
	subscription string
	pending      *streamDelivery
}

func (s *latestState) reset(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscription = id
	s.pending = nil
}
func (s *latestState) put(value streamDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscription == value.subscription {
		s.pending = &value
	}
}
func (s *latestState) take() *streamDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	value := s.pending
	s.pending = nil
	return value
}
