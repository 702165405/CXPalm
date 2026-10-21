package desktop

import (
	"encoding/json"
	"errors"
)

func approvalFingerprint(raw map[string]any, requestID string) string {
	requests, _ := raw["requests"].([]any)
	for _, entry := range requests {
		if request, ok := entry.(map[string]any); ok && fmtID(request["id"]) == requestID {
			encoded, _ := json.Marshal(request)
			return string(encoded)
		}
	}
	return ""
}

// Resolve decisions against the owner's current request, never client-supplied grants.
func approvalAction(raw map[string]any, thread, requestID string, decision json.RawMessage) (string, map[string]any, error) {
	var choice string
	if json.Unmarshal(decision, &choice) != nil || (choice != "accept" && choice != "decline" && choice != "cancel") {
		return "", nil, errors.New("unsupported approval decision")
	}
	requests, _ := raw["requests"].([]any)
	for _, entry := range requests {
		request, ok := entry.(map[string]any)
		if !ok || requestID == "" || fmtID(request["id"]) != requestID {
			continue
		}
		params, _ := request["params"].(map[string]any)
		if id, ok := params["threadId"].(string); ok && id != thread {
			return "", nil, errors.New("approval belongs to another conversation")
		}
		payload := map[string]any{"requestId": request["id"]}
		switch request["method"] {
		case "item/permissions/requestApproval":
			if choice == "cancel" {
				return "", nil, errors.New("unsupported approval decision")
			}
			permissions, ok := params["permissions"].(map[string]any)
			if !ok || params["threadId"] != thread {
				return "", nil, errors.New("invalid permission request")
			}
			// Scope is deliberately limited to this turn; no session or permanent grants.
			granted := map[string]any{}
			if choice == "accept" {
				for key, value := range permissions {
					if key != "network" && key != "fileSystem" {
						return "", nil, errors.New("unsupported permission profile; use official desktop")
					}
					granted[key] = value
				}
			}
			payload["response"] = map[string]any{"permissions": granted, "scope": "turn"}
			return "thread-follower-permissions-request-approval-response", payload, nil
		case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
			if available, exists := params["availableDecisions"]; exists {
				choices, ok := available.([]any)
				allowed := false
				for _, candidate := range choices {
					if s, ok := candidate.(string); ok && s == choice {
						allowed = true
					}
				}
				if !ok || !allowed {
					return "", nil, errors.New("decision is not offered by this approval")
				}
			}
			payload["decision"] = choice
			method := "thread-follower-command-approval-decision"
			if request["method"] == "item/fileChange/requestApproval" {
				method = "thread-follower-file-approval-decision"
			}
			return method, payload, nil
		default:
			return "", nil, errors.New("this request must be handled in official desktop")
		}
	}
	return "", nil, errors.New("approval request has expired")
}
