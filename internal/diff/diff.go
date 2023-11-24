// Package diff provides utilities for comparing strings and slices
// to produce a readable diff output for tests.
package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// Lines returns a diff of two strings, line-by-line.
func Lines(want, got string) string {
	return Diff(strings.Split(want, "\n"), strings.Split(got, "\n"))
}

// Diff is a silly diff implementation
// that compares the provided slices and returns a diff of them.
func Diff[T comparable](want, got []T) string {
	// We want to pad diff output with line number in the format:
	//
	//   - 1 | line 1
	//   + 2 | line 2
	//
	// To do that, we need to know the longest line number.
	longest := max(len(want), len(got))
	lineFormat := fmt.Sprintf("%%s %%-%dd | %%v\n", len(strconv.Itoa(longest))) // e.g. "%-2d | %s%v\n"
	const (
		minus = "-"
		plus  = "+"
		equal = " "
	)

	var buf strings.Builder
	writeLine := func(idx int, kind string, v T) {
		fmt.Fprintf(&buf, lineFormat, kind, idx+1, v)
	}

	var lastEqs []T
	for i := 0; i < len(want) || i < len(got); i++ {
		if i < len(want) && i < len(got) && want[i] == got[i] {
			lastEqs = append(lastEqs, want[i])
			continue
		}

		// If there are any equal lines before this, show up to 3 of them.
		if len(lastEqs) > 0 {
			start := max(len(lastEqs)-3, 0)
			for j, eq := range lastEqs[start:] {
				writeLine(i-3+j, equal, eq)
			}
		}

		if i < len(want) {
			writeLine(i, minus, want[i])
		}
		if i < len(got) {
			writeLine(i, plus, got[i])
		}

		lastEqs = nil
	}

	return buf.String()
}
