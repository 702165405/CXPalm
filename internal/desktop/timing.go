package desktop

import "math"

// Normalize official timing variants without changing the native patch base.
func projectTiming(turn, out map[string]any) {
	number := func(key string) (float64, bool) {
		n, ok := turn[key].(float64)
		return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= 8640000000000000
	}
	start, hasStart := number("turnStartedAtMs")
	if hasStart {
		out["turnStartedAtMs"] = start
	}
	switch turn["status"] {
	case "completed", "failed", "interrupted":
		duration, ok := number("durationMs")
		if !ok {
			duration, ok = number("turnDurationMs")
		}
		if !ok {
			end, hasEnd := number("turnCompletedAtMs")
			if hasStart && hasEnd && end >= start {
				duration, ok = end-start, true
			}
		}
		if ok {
			out["durationMs"] = duration
		}
	}
}
