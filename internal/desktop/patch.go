package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func normalizePatches(raw json.RawMessage) ([]byte, error) {
	var patches []map[string]any
	if e := json.Unmarshal(raw, &patches); e != nil {
		return nil, e
	}
	for _, p := range patches {
		if p["op"] != "add" && p["op"] != "replace" && p["op"] != "remove" {
			return nil, errors.New("unsupported desktop patch operation")
		}
		switch path := p["path"].(type) {
		case string:
			if path != "" && !strings.HasPrefix(path, "/") {
				return nil, errors.New("invalid patch pointer")
			}
		case []any:
			tokens := make([]string, len(path))
			for i, value := range path {
				switch value.(type) {
				case string, float64:
				default:
					return nil, errors.New("invalid patch path token")
				}
				tokens[i] = strings.ReplaceAll(strings.ReplaceAll(fmt.Sprint(value), "~", "~0"), "/", "~1")
			}
			p["path"] = ""
			if len(tokens) > 0 {
				p["path"] = "/" + strings.Join(tokens, "/")
			}
		default:
			return nil, errors.New("invalid patch path")
		}
	}
	return json.Marshal(patches)
}
