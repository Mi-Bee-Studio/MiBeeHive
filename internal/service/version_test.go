package service

import (
	"reflect"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"22.04", []int{22, 4}},
		{"24.04.1", []int{24, 4, 1}},
		{"9-stream", []int{9}},
		{"v3.21", []int{3, 21}},
		{"10", []int{10}},
		{"", nil},
		{"ubuntu-22.04.3-live-server-arm64", []int{22, 4, 3, 64}},
		{"0.1", []int{0, 1}},
		{"1.2.3.4.5", []int{1, 2, 3, 4, 5}},
		{"no-digits", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseVersion(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name string
		a    []int
		b    []int
		want int
	}{
		{"24.4 > 22.4", []int{24, 4}, []int{22, 4}, 1},
		{"10 > 9", []int{10}, []int{9}, 1},
		{"3.22 > 3.21", []int{3, 22}, []int{3, 21}, 1},
		{"equal 3.21 == 3.21", []int{3, 21}, []int{3, 21}, 0},
		{"3.21 < 3.22", []int{3, 21}, []int{3, 22}, -1},
		{"1 < 2", []int{1}, []int{2}, -1},
		{"9 < 10", []int{9}, []int{10}, -1},
		{"nil == nil", nil, nil, 0},
		{"nil < [1]", nil, []int{1}, -1},
		{"[1] > nil", []int{1}, nil, 1},
		{"empty == empty", []int{}, []int{}, 0},
		{"empty < [1]", []int{}, []int{1}, -1},
		{"[1] > empty", []int{1}, []int{}, 1},
		// Shorter is "less" when all shared elements equal
		{"3.21 < 3.21.1", []int{3, 21}, []int{3, 21, 1}, -1},
		{"3.21.1 > 3.21", []int{3, 21, 1}, []int{3, 21}, 1},
		{"24.4.1 > 24.4", []int{24, 4, 1}, []int{24, 4}, 1},
		// Negative numbers — not expected from ParseVersion but handle gracefully
		{"negatives: -1 vs 0", []int{-1}, []int{0}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersion(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersion(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
