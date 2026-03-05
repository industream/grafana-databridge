package plugin

import (
	"testing"
)

func TestSplitIds(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"abc", []string{"abc"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitIds(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("item %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}
