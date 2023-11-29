package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"braces.dev/errtrace/internal/diff"
)

// TestGolden runs errtrace on all .go files inside testdata/golden,
// and compares the output to the corresponding .golden file.
// Files must match exactly.
//
// If log messages are expected associated with specific lines,
// they can be included in the source and the .golden file
// in the format:
//
//	foo() // want:"log message"
//
// The log message will be matched against the output of errtrace on stderr.
// The string must be a valid Go string literal.
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

	wantLogs, err := extractLogs(giveSrc)
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

	// If the source is expected to change,
	// also verify that running with -l lists the file.
	// Otherwise, verify that running with -l does not list the file.
	t.Run("list", func(t *testing.T) {
		var out bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: &out,
			Stderr: io.Discard,
		}).Run([]string{"-l", srcPath})
		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}

		if bytes.Equal(giveSrc, wantSrc) {
			if want, got := "", out.String(); got != want {
				t.Errorf("expected no output, got:\n%s", indent(got))
			}
		} else {
			if want, got := srcPath+"\n", out.String(); got != want {
				t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
			}
		}
	})

	var stdout, stderr bytes.Buffer
	defer func() {
		if t.Failed() {
			t.Logf("stdout:\n%s", indent(stdout.String()))
			t.Logf("stderr:\n%s", indent(stderr.String()))
		}
	}()

	exitCode := (&mainCmd{
		Stdout: &stdout, // We don't care about stdout.
		Stderr: &stderr,
	}).Run([]string{"-format=never", "-w", srcPath})

	if want := 0; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	gotSrc, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := string(wantSrc), string(gotSrc); got != want {
		t.Errorf("want output:\n%s\ngot:\n%s\ndiff:\n%s", indent(want), indent(got), indent(diff.Lines(want, got)))
	}

	// Check that the log messages match.
	gotLogs, err := parseLogOutput(srcPath, stderr.String())
	if err != nil {
		t.Fatal(err)
	}

	if diff := diff.Diff(wantLogs, gotLogs); diff != "" {
		t.Errorf("log messages differ:\n%s", indent(diff))
	}

	// Re-run on the output of the first run.
	// This should be a no-op.
	t.Run("idempotent", func(t *testing.T) {
		var got bytes.Buffer
		exitCode := (&mainCmd{
			Stderr: io.Discard,
			Stdout: &got,
		}).Run([]string{srcPath})

		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}

		gotSrc := got.String()
		if want, got := string(wantSrc), gotSrc; got != want {
			t.Errorf("want output:\n%s\ngot:\n%s\ndiff:\n%s", indent(want), indent(got), indent(diff.Lines(want, got)))
		}
	})
}

func TestParseFormatFlag(t *testing.T) {
	tests := []struct {
		name string
		give []string
		want format
	}{
		{
			name: "default",
			want: formatAuto,
		},
		{
			name: "auto explicit",
			give: []string{"-format=auto"},
			want: formatAuto,
		},
		{
			name: "always",
			give: []string{"-format=always"},
			want: formatAlways,
		},
		{
			name: "always explicit",
			give: []string{"-format"},
			want: formatAlways,
		},
		{
			name: "never",
			give: []string{"-format=never"},
			want: formatNever,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			flag.SetOutput(io.Discard)

			var got format
			flag.Var(&got, "format", "")
			if err := flag.Parse(tt.give); err != nil {
				t.Fatal(err)
			}

			if want, got := tt.want, got; got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestFormatFlagError(t *testing.T) {
	flag := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	flag.SetOutput(io.Discard)

	var got format
	flag.Var(&got, "format", "")
	err := flag.Parse([]string{"-format=unknown"})
	if err == nil {
		t.Fatal("no error")
	}

	if want, got := `invalid format "unknown"`, err.Error(); !strings.Contains(got, want) {
		t.Errorf("error %q does not contain %q", got, want)
	}
}

func TestFormatFlagString(t *testing.T) {
	tests := []struct {
		give format
		want string
	}{
		{formatAuto, "auto"},
		{formatAlways, "always"},
		{formatNever, "never"},
		{format(999), "format(999)"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.give), func(t *testing.T) {
			if want, got := tt.want, tt.give.String(); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestShouldFormat(t *testing.T) {
	tests := []struct {
		name string
		give mainParams
		want bool
	}{
		{"auto/no write", mainParams{Format: formatAuto}, false},
		{"auto/write", mainParams{Format: formatAuto, Write: true}, true},
		{"always", mainParams{Format: formatAlways}, true},
		{"never", mainParams{Format: formatNever}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if want, got := tt.want, tt.give.shouldFormat(); got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Fatal("no panic")
			}
		}()

		(&mainParams{Format: format(999)}).shouldFormat()
	})
}

// -format=auto should format the file if used with -w,
// and not format the file if used without -w.
func TestFormatAuto(t *testing.T) {
	give := strings.Join([]string{
		"package foo",
		`import "errors"`,
		"func foo() error {",
		`	return errors.New("foo")`,
		"}",
	}, "\n")

	wantUnformatted := strings.Join([]string{
		"package foo",
		`import "errors"; import "braces.dev/errtrace"`,
		"func foo() error {",
		`	return errtrace.Wrap(errors.New("foo"))`,
		"}",
	}, "\n")

	wantFormatted := strings.Join([]string{
		"package foo",
		"",
		`import "errors"`,
		`import "braces.dev/errtrace"`,
		"",
		"func foo() error {",
		`	return errtrace.Wrap(errors.New("foo"))`,
		"}",
		"",
	}, "\n")

	t.Run("write", func(t *testing.T) {
		srcPath := filepath.Join(t.TempDir(), "src.go")
		if err := os.WriteFile(srcPath, []byte(give), 0o600); err != nil {
			t.Fatal(err)
		}

		exitCode := (&mainCmd{
			Stdout: io.Discard,
			Stderr: io.Discard,
		}).Run([]string{"-w", srcPath})
		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}

		bs, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}

		if want, got := wantFormatted, string(bs); got != want {
			t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
		}
	})

	t.Run("stdout", func(t *testing.T) {
		srcPath := filepath.Join(t.TempDir(), "src.go")
		if err := os.WriteFile(srcPath, []byte(give), 0o600); err != nil {
			t.Fatal(err)
		}

		var out bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: &out,
			Stderr: io.Discard,
		}).Run([]string{srcPath})
		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}

		if want, got := wantUnformatted, out.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
		}
	})

	t.Run("stdin", func(t *testing.T) {
		var out bytes.Buffer
		exitCode := (&mainCmd{
			Stdin:  strings.NewReader(give),
			Stdout: &out,
			Stderr: io.Discard,
		}).Run(nil /* args */) // empty args implies stdin
		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}
		if want, got := wantUnformatted, out.String(); want != got {
			t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
		}
	})

	t.Run("stdin incompatible with write", func(t *testing.T) {
		var err, out bytes.Buffer
		exitCode := (&mainCmd{
			Stdin:  strings.NewReader("unused"),
			Stdout: &out,
			Stderr: &err,
		}).Run([]string{"-w"})
		if want := 1; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}
		if want, got := "", out.String(); want != got {
			t.Errorf("stdout = %q, want %q", got, want)
		}
		if want, got := "-:can't use -w with stdin\n", err.String(); want != got {
			t.Errorf("stderr = %q, want %q", got, want)
		}
	})
}

func TestListFlag(t *testing.T) {
	uninstrumentedSource := strings.Join([]string{
		"package foo",
		`import "errors"`,
		"func foo() error {",
		`	return errors.New("foo")`,
		"}",
	}, "\n")

	instrumentedSource := strings.Join([]string{
		"package foo",
		`import "errors"; import "braces.dev/errtrace"`,
		"func foo() error {",
		`	return errtrace.Wrap(errors.New("foo"))`,
		"}",
	}, "\n")

	dir := t.TempDir()

	instrumented := filepath.Join(dir, "instrumented.go")
	if err := os.WriteFile(instrumented, []byte(instrumentedSource), 0o600); err != nil {
		t.Fatal(err)
	}

	uninstrumented := filepath.Join(dir, "uninstrumented.go")
	if err := os.WriteFile(uninstrumented, []byte(uninstrumentedSource), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	exitCode := (&mainCmd{
		Stdout: &out,
		Stderr: io.Discard,
	}).Run([]string{"-l", uninstrumented, instrumented})
	if want := 0; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	// Only the uninstrumented file should be listed.
	if want, got := uninstrumented+"\n", out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
	}
}

func indent(s string) string {
	return "\t" + strings.ReplaceAll(s, "\n", "\n\t")
}

type logLine struct {
	Line int
	Msg  string
}

// extractLogs parses the "// want" comments in src
// into a slice of logLine structs.
func extractLogs(src []byte) ([]logLine, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var logs []logLine
	for _, c := range f.Comments {
		for _, l := range c.List {
			if !strings.HasPrefix(l.Text, "// want:") {
				continue
			}

			pos := fset.Position(l.Pos())
			lit := strings.TrimPrefix(l.Text, "// want:")

			s, err := strconv.Unquote(lit)
			if err != nil {
				return nil, fmt.Errorf("%s:bad string literal: %s", pos, lit)
			}

			logs = append(logs, logLine{Line: pos.Line, Msg: s})
		}
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Line < logs[j].Line
	})

	return logs, nil
}

func parseLogOutput(file, s string) ([]logLine, error) {
	var logs []logLine
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}

		// Drop the path so we can determinstically split on ":" (which is a valid character in Windows paths).
		line = strings.TrimPrefix(line, file)
		parts := strings.SplitN(line, ":", 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("bad log line: %q", line)
		}

		var msg string
		switch len(parts) {
		case 3:
			msg = parts[2] // file:line:msg
		case 4:
			msg = parts[3] // file:line:column:msg
		}

		lineNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("bad log line: %q", line)
		}

		logs = append(logs, logLine{
			Line: lineNum,
			Msg:  msg,
		})
	}

	return logs, nil
}
