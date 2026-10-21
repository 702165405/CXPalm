package server

import (
	"encoding/json"
	"regexp"

	"cxpalm/internal/desktop"
	"cxpalm/internal/store"
)

var legacyArchiveWriter = regexp.MustCompile(`^official metadata API: thread ([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}) already has an active writer$`)

func (s *Server) resolveOperation(device, id string) (store.Receipt, error) {
	r, err := s.Store.ResolveReceipt(device, id)
	if err != nil || r.Status != "unknown" {
		return r, err
	}
	var result struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(r.Result, &result) != nil {
		return r, nil
	}
	match := legacyArchiveWriter.FindStringSubmatch(result.Error)
	if match == nil {
		return r, nil
	}
	// An error string alone is not proof: creating a chat can fail after partial
	// success. Verify the exact archive command fingerprint before releasing it.
	for _, method := range []string{"sessions.archive", "sessions.unarchive"} {
		command := Command{ID: id, Method: method, Thread: match[1]}
		encoded, _ := json.Marshal(command)
		ok, err := s.Store.MatchesOperation(device, id, store.Hash(string(encoded)))
		if err != nil {
			return r, err
		}
		if ok {
			r.Status = "rejected"
			r.Result, _ = json.Marshal(map[string]string{"error": desktop.ArchiveWriterMessage, "code": "active_writer", "method": method, "thread": match[1]})
			return r, nil
		}
	}
	return r, nil
}
