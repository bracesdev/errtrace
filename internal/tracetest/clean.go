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

// MustClean cleans the trace, panicking if it cannot.
// See [Clean] for details.
func MustClean(trace string) string {
	cleaned, err := Clean(trace)
	if err != nil {
		panic(err)
	}
	return cleaned
}

const _fixedDir = "/path/to/errtrace"

// _fileLineMatcher matches file:line where file starts with the fixedDir.
// Capture groups:
//
//  1. file path
//  2. line number
var _fileLineMatcher = regexp.MustCompile("(" + regexp.QuoteMeta(_fixedDir) + `[^:]+):(\d+)`)

// Clean makes traces more deterministic for tests by:
//
//   - replacing the environment-specific path to errtrace
//     with the fixed path /path/to/errtrace
//   - replacing line numbers with the lowest values
//     that maintain relative ordering within the file
func Clean(trace string) (string, error) {
	errtraceDir := getErrtraceDir()
	trace = strings.ReplaceAll(trace, errtraceDir, _fixedDir)

	replacer := newFileLineReplacer()
	for _, m := range _fileLineMatcher.FindAllStringSubmatch(trace, -1) {
		file := m[1]
		lineStr := m[2]
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			panic(fmt.Sprintf("file:line regex matched invalid line number: %v", err))
		}

		replacer.add(file, line)
	}

	replacements := replacer.replacements()
	trace = _fileLineMatcher.ReplaceAllStringFunc(trace, func(s string) string {
		return replacements[s]
	})

	return trace, nil
}

func getErrtraceDir() string {
	_, file, _, _ := runtime.Caller(0)
	// Note: Assumes specific location of this file in errtrace, strip internal/tracetest/<file>
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

type fileLineReplacer struct {
	fileLines map[string][]int
}

func newFileLineReplacer() *fileLineReplacer {
	return &fileLineReplacer{
		fileLines: make(map[string][]int),
	}
}

func (r *fileLineReplacer) add(file string, line int) {
	r.fileLines[file] = append(r.fileLines[file], line)
}

func (r *fileLineReplacer) replacements() map[string]string {
	allReplacements := make(map[string]string)
	for file := range r.fileLines {
		replacements := lineReplacements(r.fileLines[file])
		for origLine, replaceLine := range replacements {
			allReplacements[fmt.Sprintf("%v:%v", file, origLine)] = fmt.Sprintf("%v:%v", file, replaceLine)
		}
	}
	return allReplacements
}

func lineReplacements(v []int) map[int]int {
	if len(v) == 0 {
		return nil
	}
	sort.Ints(v)

	replacements := make(map[int]int)
	last := -1 // not a valid line number (neither is 0, but -1 is defensive)
	next := 1
	for _, v := range v {
		if v != last {
			replacements[v] = next
			next += 1
		}
		last = v
	}
	return replacements
}
