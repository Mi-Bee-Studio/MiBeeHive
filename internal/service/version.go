package service

import (
	"regexp"
	"strconv"
)

var digitPattern = regexp.MustCompile(`\d+`)

// ParseVersion extracts all numeric sequences from a version string.
// Returns the integer values in order of appearance.
// Returns nil if no digits are found.
//
// Examples:
//
//	ParseVersion("22.04")       → []int{22, 4}
//	ParseVersion("24.04.1")     → []int{24, 4, 1}
//	ParseVersion("9-stream")    → []int{9}
//	ParseVersion("v3.21")       → []int{3, 21}
//	ParseVersion("")            → nil
func ParseVersion(s string) []int {
	matches := digitPattern.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}
	parts := make([]int, len(matches))
	for i, m := range matches {
		n, _ := strconv.Atoi(m)
		parts[i] = n
	}
	return parts
}

// CompareVersion performs numeric tuple comparison on two version slices.
// Returns:
//
//	 1 if a > b
//	 0 if a == b
//	-1 if a < b
//
// Shorter slices are considered "less" when all shared elements are equal.
// nil and empty slices are treated as equal to each other and less than any non-empty slice.
func CompareVersion(a, b []int) int {
	// Handle nil/empty cases
	aLen := len(a)
	bLen := len(b)

	if aLen == 0 && bLen == 0 {
		return 0
	}
	if aLen == 0 {
		return -1
	}
	if bLen == 0 {
		return 1
	}

	// Compare element by element
	minLen := aLen
	if bLen < minLen {
		minLen = bLen
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	// All shared elements equal; shorter is "less"
	if aLen < bLen {
		return -1
	}
	if aLen > bLen {
		return 1
	}
	return 0
}
