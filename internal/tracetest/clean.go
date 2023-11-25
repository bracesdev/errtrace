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

// MustClean makes traces more deterministic for tests by:
// 1. Replacing the environment-specific path to errtrace with a fixed string.
// 2. Replacing line numbers with the lowest values that maintain relative ordering within the file.
func MustClean(trace string) string {
	const fixedDir = "/path/to/errtrace"
	errtraceDir := getErrtraceDir()
	trace = strings.ReplaceAll(trace, errtraceDir, fixedDir)

	// Match file:line where file starts with the fixedDir.
	fileLineMatcher := regexp.MustCompile("(" + regexp.QuoteMeta(fixedDir) + "[^:]+):([0-9]+)")

	replacer := newFileLineReplacer()
	for _, m := range fileLineMatcher.FindAllStringSubmatch(trace, -1) {
		file := m[1]
		lineStr := m[2]
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			panic(fmt.Sprintf("file:line regex matched invalid line number: %v", err))
		}

		replacer.add(file, line)
	}

	replacements := replacer.replacements()
	trace = fileLineMatcher.ReplaceAllStringFunc(trace, func(s string) string {
		return replacements[s]
	})

	return trace
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
