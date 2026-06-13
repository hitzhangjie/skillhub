// Package version check skill version
package version

import (
	"strconv"
	"strings"
)

func Parse(v string) []int {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	lower = strings.TrimPrefix(lower, "v")
	if lower == "" {
		return nil
	}
	// Strip pre-release / build metadata
	if idx := strings.IndexAny(lower, "-+"); idx != -1 {
		lower = lower[:idx]
	}
	parts := strings.Split(lower, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func IsNewer(candidate, current string) bool {
	candidate = strings.TrimSpace(candidate)
	current = strings.TrimSpace(current)
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	a := Parse(candidate)
	b := Parse(current)
	if a != nil && b != nil {
		return compareSlices(a, b) > 0
	}
	return candidate != current
}

func compareSlices(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		va, vb := 0, 0
		if i < len(a) {
			va = a[i]
		}
		if i < len(b) {
			vb = b[i]
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

func AtLeast(v string, minimum []int) bool {
	parsed := Parse(v)
	if parsed == nil {
		return false
	}
	return compareSlices(parsed, minimum) >= 0
}

func Normalize(v string) string {
	return strings.TrimSpace(v)
}
