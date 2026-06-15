package utils

import (
	"strings"
	"testing"
)

func TestResolveHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"127.0.0.1:3000", ":3000"},
		{"redis://default:password@127.0.0.1:3000", ":3000"},
		{"http://127.0.0.1:8080/some/path", ":8080"},
	}

	for _, tc := range tests {
		res, err := ResolveHost(tc.input)
		if err != nil {
			t.Errorf("failed to resolve %q: %v", tc.input, err)
			continue
		}
		if !strings.HasSuffix(res, tc.expected) {
			t.Errorf("expected suffix %q in %q for input %q", tc.expected, res, tc.input)
		}
	}
}
