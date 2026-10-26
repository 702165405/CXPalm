package desktop

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
)

// Immutable native trees share untouched branches. JSON bytes are materialized only for legacy consumers.
type nativeDocument struct {
	value   map[string]any
	once    sync.Once
	encoded json.RawMessage
}

func (d *nativeDocument) bytes() json.RawMessage {
	d.once.Do(func() {
		if d.encoded == nil {
			d.encoded = mustJSON(d.value)
		}
	})
	return d.encoded
}
func (s State) hasSnapshot() bool { return s.document != nil || len(s.Snapshot) > 0 }
func (s State) snapshotJSON() json.RawMessage {
	if s.document != nil {
		return s.document.bytes()
	}
	return s.Snapshot
}
func (s State) snapshotValue() map[string]any {
	if s.document != nil {
		return s.document.value
	}
	var value map[string]any
	_ = json.Unmarshal(s.Snapshot, &value)
	return value
}
func patchDocument(root map[string]any, encoded json.RawMessage) (map[string]any, error) {
	var operations []struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(encoded, &operations); err != nil {
		return nil, err
	}
	var result any = root
	for _, operation := range operations {
		if operation.Op != "add" && operation.Op != "replace" && operation.Op != "remove" {
			return nil, errors.New("unsupported native patch")
		}
		var path []string
		if operation.Path != "" {
			if !strings.HasPrefix(operation.Path, "/") {
				return nil, errors.New("invalid pointer")
			}
			for _, part := range strings.Split(operation.Path[1:], "/") {
				var key strings.Builder
				for i := 0; i < len(part); i++ {
					if part[i] != '~' {
						key.WriteByte(part[i])
						continue
					}
					i++
					if i >= len(part) || part[i] != '0' && part[i] != '1' {
						return nil, errors.New("invalid pointer escape")
					}
					if part[i] == '0' {
						key.WriteByte('~')
					} else {
						key.WriteByte('/')
					}
				}
				path = append(path, key.String())
			}
		}
		if len(path) > 128 {
			return nil, errors.New("native patch too deep")
		}
		var value any
		if operation.Op != "remove" {
			if len(operation.Value) == 0 {
				return nil, errors.New("patch value required")
			}
			if err := json.Unmarshal(operation.Value, &value); err != nil {
				return nil, err
			}
		}
		var visit func(any, int) (any, error)
		visit = func(current any, depth int) (any, error) {
			if depth == len(path) {
				if operation.Op == "remove" {
					return nil, errors.New("cannot remove native root")
				}
				return value, nil
			}
			key := path[depth]
			last := depth == len(path)-1
			switch node := current.(type) {
			case map[string]any:
				old, exists := node[key]
				if !exists && (!last || operation.Op != "add") {
					return nil, errors.New("missing native object key")
				}
				copy := make(map[string]any, len(node)+1)
				for k, v := range node {
					copy[k] = v
				}
				if last && operation.Op == "remove" {
					delete(copy, key)
					return copy, nil
				}
				next, err := visit(old, depth+1)
				if err != nil {
					return nil, err
				}
				copy[key] = next
				return copy, nil
			case []any:
				index, err := strconv.Atoi(key)
				if key == "-" && last && operation.Op == "add" {
					index = len(node)
					err = nil
				}
				if err != nil || index < 0 || strconv.Itoa(index) != key && key != "-" || index > len(node) || index == len(node) && (!last || operation.Op != "add") {
					return nil, errors.New("invalid native array index")
				}
				if last && operation.Op == "add" {
					copy := make([]any, len(node)+1)
					copy = append(copy[:0], node[:index]...)
					copy = append(copy, value)
					copy = append(copy, node[index:]...)
					return copy, nil
				}
				if last && operation.Op == "remove" {
					copy := append([]any{}, node[:index]...)
					copy = append(copy, node[index+1:]...)
					return copy, nil
				}
				copy := append([]any{}, node...)
				next, err := visit(node[index], depth+1)
				if err != nil {
					return nil, err
				}
				copy[index] = next
				return copy, nil
			default:
				return nil, errors.New("native patch parent is not a container")
			}
		}
		next, err := visit(result, 0)
		if err != nil {
			return nil, err
		}
		result = next
	}
	value, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("native root must remain an object")
	}
	return value, nil
}

// Notifications need only runtime/request identities, never megabytes of tools or prose.
func NotificationState(s State) State {
	raw := s.snapshotValue()
	s.document = nil
	if raw == nil {
		s.Snapshot = nil
		return s
	}
	s.ReadToken = readToken(raw)
	requests := []any{}
	for _, entry := range orderedItems(raw["requests"]) {
		if request, ok := entry.(map[string]any); ok {
			requests = append(requests, map[string]any{"id": request["id"]})
		}
	}
	identity := map[string]any{}
	turns := orderedTurns(raw)
	if len(turns) > 0 {
		if turn, ok := turns[len(turns)-1].(map[string]any); ok {
			for _, key := range []string{"turnId", "id", "status", "turnStartedAtMs", "turnCompletedAtMs"} {
				identity[key] = turn[key]
			}
		}
	}
	identity["index"] = len(turns) - 1
	s.Snapshot = mustJSON(map[string]any{"threadRuntimeStatus": raw["threadRuntimeStatus"], "requests": requests, "turnIdentity": identity})
	return s
}
