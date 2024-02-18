package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/diff"
)

func TestToolExec(t *testing.T) {
	const testProg = "./testdata/toolexec-test"

	errTraceCmd := filepath.Join(t.TempDir(), "errtrace")
	runGo(t, ".", "build", "-o", errTraceCmd, ".")

	var wantTraces []string
	err := filepath.Walk(testProg, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return errtrace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}

		for _, line := range findTraceLines(t, path) {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Fatalf("abspath: %v", err)
			}

			wantTraces = append(wantTraces, fmt.Sprintf("%v:%v", absPath, line))
		}
		return nil
	})
	if err != nil {
		t.Fatal("Walk failed", err)
	}
	sort.Strings(wantTraces)

	t.Run("no toolexec", func(t *testing.T) {
		stdout, _ := runGo(t, testProg, "run", ".")
		if lines := fileLines(stdout); len(lines) > 0 {
			t.Errorf("expected no file:line, got %v", lines)
		}
	})

	t.Run("with toolexec", func(t *testing.T) {
		stdout, _ := runGo(t, testProg, "run", "-toolexec", errTraceCmd, ".")
		gotLines := fileLines(stdout)

		sort.Strings(gotLines)
		if d := diff.Diff(wantTraces, gotLines); d != "" {
			t.Errorf("diff in traces:\n%s", d)
			t.Errorf("go run output:\n%s", stdout)
		}
	})
}

func findTraceLines(t testing.TB, file string) []int {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	var traces []int
	scanner := bufio.NewScanner(f)
	var lineNum int
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, "// @trace") {
			traces = append(traces, lineNum)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	return traces
}

var fileLineRegex = regexp.MustCompile(`^\s*(.*:[0-9]+)$`)

func fileLines(out string) []string {
	var fileLines []string
	for _, line := range strings.Split(out, "\n") {
		if fileLineRegex.MatchString(line) {
			fileLines = append(fileLines, strings.TrimSpace(line))
		}
	}
	return fileLines
}

func runGo(t testing.TB, dir string, args ...string) (stdout, stderr string) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	return stdoutBuf.String(), stderrBuf.String()
}
