package utils

import (
	"testing"
)

func TestCompareValues(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{a: "abc", b: "abc", expected: 0},
		{a: "", b: "abc", expected: 1},
		{a: "abc", b: "", expected: -1},
		{a: "9", b: "10", expected: -1},
		{a: "1.5", b: "1.25", expected: 1},
		{a: "-3", b: "2", expected: -1},
		{a: "10.0.0.9", b: "10.0.0.10", expected: -1},
		{a: "t3.2xlarge", b: "t3.12xlarge", expected: -1},
		{a: "Running", b: "stopped", expected: -1},
		{a: "web-02", b: "web-2", expected: -1},
		{a: "web-1", b: "web-1a", expected: -1},
		{a: "2024-01-02 10:00:00", b: "2023-12-31 23:59:59", expected: 1},
		{a: "B", b: "b", expected: -1},
	}

	for _, test := range tests {
		got := CompareValues(test.a, test.b)
		if got != test.expected {
			t.Errorf("CompareValues(%q, %q): expected %v, got %v", test.a, test.b, test.expected, got)
		}
		if rev := CompareValues(test.b, test.a); rev != -test.expected {
			t.Errorf("CompareValues(%q, %q): expected %v, got %v", test.b, test.a, -test.expected, rev)
		}
	}
}
