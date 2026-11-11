package main

import (
	"reflect"
	"testing"
)

func TestParseContexts(t *testing.T) {
	tests := []struct {
		input string
		want  []string
		bad   bool
	}{
		{"", nil, false},
		{"4,16", []string{"4k", "16k"}, false},
		{" 4, 16,32 ", []string{"4k", "16k", "32k"}, false},
		{"128", []string{"128k"}, false},
		{"4,4", nil, true},
		{"4,12", nil, true},
		{",4", nil, true},
	}
	for _, tt := range tests {
		got, err := parseContexts(tt.input)
		if (err != nil) != tt.bad {
			t.Errorf("parseContexts(%q) error = %v, want error %v", tt.input, err, tt.bad)
			continue
		}
		if !tt.bad && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseContexts(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
