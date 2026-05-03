package project

import (
	"strings"
	"testing"
)

func TestRedactStringTokenPatterns(t *testing.T) {
	longToken := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	cases := []struct {
		name      string
		input     string
		want      string
		mustNot   string
		unchanged bool
	}{
		{name: "bearer token", input: "Bearer " + longToken, want: "Bearer " + RedactedPlaceholder, mustNot: longToken},
		{name: "authorization assignment", input: "Authorization: Bearer " + longToken, want: "Authorization: " + RedactedPlaceholder, mustNot: longToken},
		{name: "api token assignment", input: "api_token=" + longToken, want: "api_token: " + RedactedPlaceholder, mustNot: longToken},
		{name: "secret assignment", input: "secret:" + longToken, want: "secret: " + RedactedPlaceholder, mustNot: longToken},
		{name: "long token-like value", input: "value " + longToken + " done", want: RedactedPlaceholder, mustNot: longToken},
		{name: "url false positive", input: "see https://example.com/path/that/is/long/enough/to/match/token/pattern", unchanged: true},
		{name: "uuid false positive", input: "run 123e4567-e89b-12d3-a456-426614174000", unchanged: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactString(tt.input)
			if tt.unchanged {
				if got != tt.input {
					t.Fatalf("RedactString() = %q, want unchanged %q", got, tt.input)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("RedactString() = %q, want contains %q", got, tt.want)
			}
			if strings.Contains(got, tt.mustNot) {
				t.Fatalf("RedactString() leaked %q in %q", tt.mustNot, got)
			}
		})
	}
}

func TestRedactValueRecursesThroughNestedJSON(t *testing.T) {
	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	input := map[string]any{
		"api_token": secret,
		"nested": map[string]any{
			"password": "hunter2",
			"notes":    "Bearer " + secret,
		},
		"items": []any{
			map[string]any{"authorization": "Bearer " + secret},
			"plain text",
		},
		"safe": "https://example.com/path/that/is/long/enough/to/match/token/pattern",
	}

	got, ok := RedactValue(input).(map[string]any)
	if !ok {
		t.Fatalf("RedactValue() = %T, want map", RedactValue(input))
	}
	if got["api_token"] != RedactedPlaceholder {
		t.Fatalf("api_token = %#v, want redacted", got["api_token"])
	}
	nested := got["nested"].(map[string]any)
	if nested["password"] != RedactedPlaceholder {
		t.Fatalf("password = %#v, want redacted", nested["password"])
	}
	if strings.Contains(nested["notes"].(string), secret) || !strings.Contains(nested["notes"].(string), RedactedPlaceholder) {
		t.Fatalf("notes = %#v, want bearer secret redacted", nested["notes"])
	}
	items := got["items"].([]any)
	first := items[0].(map[string]any)
	if first["authorization"] != RedactedPlaceholder {
		t.Fatalf("authorization = %#v, want redacted", first["authorization"])
	}
	if got["safe"] != input["safe"] {
		t.Fatalf("safe = %#v, want unchanged", got["safe"])
	}
}
