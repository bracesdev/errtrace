package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/golden/*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".go")
		t.Run(name, func(t *testing.T) {
			testGolden(t, file)
		})
	}
}

func testGolden(t *testing.T, file string) {
	giveSrc, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	wantSrc, err := os.ReadFile(file + ".golden")
	if err != nil {
		t.Fatal("Bad test: missing .golden file:", err)
	}

	// Copy into a temporary directory so that we can run with -w.
	srcPath := filepath.Join(t.TempDir(), "src.go")
	if err := os.WriteFile(srcPath, []byte(giveSrc), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	defer func() {
		if t.Failed() {
			t.Logf("output:\n%s", output.String())
		}
	}()

	exitCode := (&mainCmd{
		Stderr: &output,
		Stdout: &output,
	}).Run([]string{"-w", srcPath})

	if want := 0; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	gotSrc, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := string(wantSrc), string(gotSrc); got != want {
		t.Errorf("want output:\n%s\ngot:\n%s\ndiff:\n%s", indent(want), indent(got), indent(diff(want, got)))
	}

	// Re-run on the output of the first run.
	// This should be a no-op.
	t.Run("idempotent", func(t *testing.T) {
		exitCode := (&mainCmd{
			Stderr: &output,
			Stdout: &output,
		}).Run([]string{"-w", srcPath})

		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}

		gotSrc, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}

		if want, got := string(wantSrc), string(gotSrc); got != want {
			t.Errorf("want output:\n%s\ngot:\n%s\ndiff:\n%s", indent(want), indent(got), indent(diff(want, got)))
		}
	})
}

func indent(s string) string {
	return "\t" + strings.ReplaceAll(s, "\n", "\n\t")
}

// diff is a very simple diff implementation
// that does a line-by-line comparison of two strings.
func diff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	// We want to pad diff output with line number in the format:
	//
	//   - 1 | line 1
	//   + 2 | line 2
	//
	// To do that, we need to know the longest line number.
	longest := max(len(wantLines), len(gotLines))
	lineFormat := fmt.Sprintf("%%s %%-%dd | %%s\n", len(strconv.Itoa(longest))) // e.g. "%-2d | %s%s\n"
	const (
		minus = "-"
		plus  = "+"
		equal = " "
	)

	var buf strings.Builder
	writeLine := func(idx int, kind, line string) {
		fmt.Fprintf(&buf, lineFormat, kind, idx+1, line)
	}

	var lastEqs []string
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		if i < len(wantLines) && i < len(gotLines) && wantLines[i] == gotLines[i] {
			lastEqs = append(lastEqs, wantLines[i])
			continue
		}

		// If there are any equal lines before this, show up to 3 of them.
		if len(lastEqs) > 0 {
			start := max(len(lastEqs)-3, 0)
			for j, eq := range lastEqs[start:] {
				writeLine(i-3+j, equal, eq)
			}
		}

		if i < len(wantLines) {
			writeLine(i, minus, wantLines[i])
		}
		if i < len(gotLines) {
			writeLine(i, plus, gotLines[i])
		}

		lastEqs = nil
	}

	return buf.String()
}
