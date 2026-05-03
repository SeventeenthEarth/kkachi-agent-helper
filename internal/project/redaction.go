package project

import (
	"regexp"
	"strings"
)

const RedactedPlaceholder = "[REDACTED]"

var (
	secretKeyPattern       = regexp.MustCompile(`(?i)(token|secret|password|passwd|api[_-]?key|api[_-]?token|authorization|credential|access[_-]?key|refresh[_-]?key|private[_-]?key|session[_-]?key)`)
	bearerValuePattern     = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]+`)
	assignmentValuePattern = regexp.MustCompile(`(?i)\b(token|secret|password|passwd|api[_-]?key|api[_-]?token|authorization|credential|access[_-]?key|refresh[_-]?key|private[_-]?key|session[_-]?key)\s*[:=]\s*([^\s,;]+)`)
	longTokenPattern       = regexp.MustCompile(`\b[A-Za-z0-9_./+~=-]{40,}\b`)
	lowercasePattern       = regexp.MustCompile(`[a-z]`)
	uppercasePattern       = regexp.MustCompile(`[A-Z]`)
	digitPattern           = regexp.MustCompile(`[0-9]`)
	symbolPattern          = regexp.MustCompile(`[._~+/=-]`)
)

// RedactString removes token-like values from diagnostic text. It intentionally
// preserves keys and surrounding text so remediation remains understandable.
func RedactString(value string) string {
	if value == "" {
		return value
	}
	value = bearerValuePattern.ReplaceAllString(value, `${1}`+RedactedPlaceholder)
	value = assignmentValuePattern.ReplaceAllString(value, `${1}: `+RedactedPlaceholder)
	value = longTokenPattern.ReplaceAllStringFunc(value, func(candidate string) string {
		if looksTokenLike(candidate) {
			return RedactedPlaceholder
		}
		return candidate
	})
	return value
}

// RedactValue recursively redacts token-like values from JSON-like diagnostic
// payloads. Map keys are preserved, but sensitive key values are replaced.
func RedactValue(value any) any {
	return redactValueWithKey("", value)
}

func redactValueWithKey(key string, value any) any {
	if secretKeyPattern.MatchString(key) {
		return RedactedPlaceholder
	}
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for k, v := range typed {
			redacted[k] = redactValueWithKey(k, v)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, v := range typed {
			redacted[i] = redactValueWithKey("", v)
		}
		return redacted
	case string:
		return RedactString(typed)
	default:
		return value
	}
}

func looksTokenLike(value string) bool {
	if strings.Contains(value, "://") || (strings.Contains(value, "/") && strings.Contains(value, ".")) {
		return false
	}
	if strings.Count(value, "-") >= 4 {
		return false
	}
	classes := 0
	for _, pattern := range []*regexp.Regexp{lowercasePattern, uppercasePattern, digitPattern, symbolPattern} {
		if pattern.MatchString(value) {
			classes++
		}
	}
	return classes >= 2
}
