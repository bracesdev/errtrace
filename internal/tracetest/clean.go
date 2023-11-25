// Package tracetest provides utilities for errtrace
// to test error trace output conveniently.
package tracetest

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const _fixedDir = "/path/to/errtrace"

// _fileLineMatcher matches file:line where file starts with the fixedDir.
// Capture groups:
//
//  1. file path
//  2. line number
var _fileLineMatcher = regexp.MustCompile("(" + regexp.QuoteMeta(_fixedDir) + `[^:]+):(\d+)`)

// MustClean makes traces more deterministic for tests by:
//
//   - replacing the environment-specific path to errtrace
//     with the fixed path /path/to/errtrace
//   - replacing line numbers with the lowest values
//     that maintain relative ordering within the file
//
// Note that lines numbers are replaced with increasing values starting at 1,
// with earlier positions in the file getting lower numbers.
// The relative ordering of lines within a file is maintained.
func MustClean(trace string) string {
	// Get deterministic file paths first.
	trace = strings.ReplaceAll(trace, getErrtraceDir(), _fixedDir)

	replacer := make(fileLineReplacer)
	for _, m := range _fileLineMatcher.FindAllStringSubmatch(trace, -1) {
		file := m[1]
		lineStr := m[2]
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			panic(fmt.Sprintf("matched bad line number in %q: %v", m[0], err))
		}
		replacer.Add(file, line)
	}

	return strings.NewReplacer(replacer.Replacements()...).Replace(trace)
}

func getErrtraceDir() string {
	_, file, _, _ := runtime.Caller(0)
	// Note: Assumes specific location of this file in errtrace, strip internal/tracetest/<file>
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// fileLineReplacer maintains a mapping from
// file name to line numbers in that file that are referenced.
// This is used to generate the replacements to be applied to the trace.
type fileLineReplacer map[string][]int

// Add adds a file:line pair to the replacer.
func (r fileLineReplacer) Add(file string, line int) {
	r[file] = append(r[file], line)
}

// Replacements generates a slice of pairs of Replacements
// to be applied to the trace.
//
// The first element in each pair is the original file:line
// and the second element is the replacement file:line.
// This returned slice can be fed into strings.NewReplacer.
func (r fileLineReplacer) Replacements() []string {
	var allReplacements []string
	for file, fileLines := range r {
		// Sort the lines in the file, and remove duplicates.
		// The result will be a slice of unique line numbers.
		// The index of each line in this slice + 1 will be its new line number.
		sort.Ints(fileLines)
		fileLines = uniq(fileLines)

		for idx, origLine := range fileLines {
			replaceLine := idx + 1
			allReplacements = append(allReplacements,
				fmt.Sprintf("%v:%v", file, origLine),
				fmt.Sprintf("%v:%v", file, replaceLine))
		}
	}
	return allReplacements
}

// uniq removes contiguous duplicates from v.
// The slice storage is re-used so the original slice
// should not be used after calling this function.
func uniq[T comparable](items []T) []T {
	if len(items) == 0 {
		return items
	}

	newItems := items[:1]
	for _, item := range items[1:] {
		if item != newItems[len(newItems)-1] {
			newItems = append(newItems, item)
		}
	}
	return newItems
}
