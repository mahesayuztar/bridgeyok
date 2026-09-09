package chat

import (
	"strings"
	"testing"
)

func TestUnicodeValidation(t *testing.T) {
	target := Target{Scope: "private", ID: "11111111-1111-4111-8111-111111111111"}
	for _, test := range []struct {
		name, content string
		valid         bool
	}{
		{"emoji", "😀 👍🏽 👨‍👩‍👧‍👦 ♠️ ♥️ ♦️ ♣️", true},
		{"grapheme boundary", strings.Repeat("👩🏽‍💻", 1000), true},
		{"too many", strings.Repeat("😀", 1001), false},
		{"combining marks", strings.Repeat("e\u0301", 1000), true},
		{"blank", " \n ", false}, {"nul", "a\x00b", false}, {"invalid UTF8", string([]byte{0xff}), false},
		{"bounded bytes", "a" + strings.Repeat("\u0301", 9000), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Validate(target, "request-123", test.content); (got == nil) != test.valid {
				t.Fatalf("valid=%v: %v", test.valid, got)
			}
		})
	}
}
