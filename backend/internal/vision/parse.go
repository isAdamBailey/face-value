package vision

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseIdentification decodes raw model output into an Identification,
// tolerating the ways a chat model tends to violate "JSON only": wrapped in
// ```json fences, or JSON embedded in surrounding prose.
func parseIdentification(raw string) (Identification, error) {
	candidate := stripFences(raw)

	var ident Identification
	if err := json.Unmarshal([]byte(candidate), &ident); err == nil {
		return ident, nil
	}

	if extracted, ok := extractJSONObject(candidate); ok {
		if err := json.Unmarshal([]byte(extracted), &ident); err == nil {
			return ident, nil
		}
	}

	return Identification{}, fmt.Errorf("no valid JSON object found in model output")
}

// stripFences removes a leading/trailing ```json (or ```) markdown fence,
// if present.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}

	s = strings.TrimPrefix(s, "```")
	if nl := strings.IndexByte(s, '\n'); nl != -1 && strings.TrimSpace(s[:nl]) != "" {
		// leading fence line was a language tag (e.g. "json")
		s = s[nl+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

// extractJSONObject returns the substring spanning the first "{" through
// its matching last "}", for output where the model wrapped the JSON in
// prose despite instructions not to.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	return s[start : end+1], true
}
