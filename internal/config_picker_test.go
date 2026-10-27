package internal

import (
	"reflect"
	"testing"
)

func TestFilterPickerItems(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
	}{
		{query: "", expected: awsRegions},
		{query: "eu-w", expected: []string{"eu-west-1", "eu-west-2", "eu-west-3"}},
		{query: "US-W", expected: []string{"us-west-1", "us-west-2"}},
		{query: "west-2", expected: []string{"eu-west-2", "us-west-2"}},
		{query: "ca", expected: []string{"ca-central-1", "ca-west-1"}},
		{query: "xyz", expected: nil},
	}

	for _, test := range tests {
		if got := filterPickerItems(awsRegions, test.query); !reflect.DeepEqual(got, test.expected) {
			t.Errorf("filterPickerItems(%q): expected %v, got %v", test.query, test.expected, got)
		}
	}
}

func TestFilterPickerItemsPrefixFirst(t *testing.T) {
	items := []string{"dev-staging", "staging", "prod"}
	if got, want := filterPickerItems(items, "stag"), []string{"staging", "dev-staging"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}
