package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
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
			Stderr: testWriter{t},
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
			Stderr: testWriter{t},
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

	// Create a Go package with the source file,
	// and run errtrace on the package.
	t.Run("package", func(t *testing.T) {
		dir := t.TempDir()

		file := filepath.Join(dir, filepath.Base(file))
		if err := os.WriteFile(file, giveSrc, 0o600); err != nil {
			t.Fatal(err)
		}

		gomod := filepath.Join(dir, "go.mod")
		pkgdir := strings.TrimSuffix(filepath.Base(file), ".go")
		importPath := path.Join("example.com/test", pkgdir)
		if err := os.WriteFile(gomod, []byte(fmt.Sprintf("module %s\ngo 1.20\n", importPath)), 0o600); err != nil {
			t.Fatal(err)
		}

		restore := chdir(t, dir)
		var stderr bytes.Buffer
		exitCode := (&mainCmd{
			Stderr: &stderr,
			Stdout: testWriter{t},
		}).Run([]string{"-format=never", "-tags=ignore", "-w", "./..."})
		if want := 0; exitCode != want {
			t.Errorf("exit code = %d, want %d", exitCode, want)
		}
		restore()

		gotSrc, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		if want, got := string(wantSrc), string(gotSrc); got != want {
			t.Errorf("want output:\n%s\ngot:\n%s\ndiff:\n%s", indent(want), indent(got), indent(diff.Lines(want, got)))
		}

		// Check that the log messages match.
		gotLogs, err := parseLogOutput(file, stderr.String())
		if err != nil {
			t.Fatal(err)
		}

		if diff := diff.Diff(wantLogs, gotLogs); diff != "" {
			t.Errorf("log messages differ:\n%s", indent(diff))
		}
	})
}

func TestParseMainParams(t *testing.T) {
	tests := []struct {
		name    string
		give    []string
		want    mainParams
		wantErr []string // non-empty if we expect an error
	}{
		{
			name: "stdin",
			want: mainParams{
				Patterns: []string{"-"},
			},
		},
		{
			name: "tags",
			give: []string{"-tags=foo, bar ,,baz ", "./..."},
			want: mainParams{
				Tags:     []string{"foo", "bar", "baz"},
				Patterns: []string{"./..."},
			},
		},
		{
			name: "errtrace tag",
			give: []string{"-tags=errtrace", "./..."},
			want: mainParams{
				// Tags: empty because errtrace is always added.
				Patterns: []string{"./..."},
			},
		},
		{
			name:    "errtrace optout",
			give:    []string{"-tags=foo,!errtrace,bar", "./..."},
			wantErr: []string{`tag "errtrace" is always set`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got mainParams
			err := got.Parse(testWriter{t}, tt.give)

			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				for _, want := range tt.wantErr {
					if got := err.Error(); !strings.Contains(got, want) {
						t.Errorf("error %q does not contain %q", got, want)
					}
				}

				return
			}

			if want, got := tt.want, got; !reflect.DeepEqual(want, got) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestCLIParseError(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := (&mainCmd{
		Stderr: &stderr,
		Stdout: testWriter{t},
	}).Run([]string{"-tags"})
	if want := 1; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	if want, got := "flag needs an argument", stderr.String(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want %q", got, want)
	}
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
			flag.SetOutput(testWriter{t})

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
	flag.SetOutput(testWriter{t})

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
			Stdout: testWriter{t},
			Stderr: testWriter{t},
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
			Stderr: testWriter{t},
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
			Stderr: testWriter{t},
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
		if want, got := "stdin:can't use -w with stdin\n", err.String(); want != got {
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
		Stderr: testWriter{t},
	}).Run([]string{"-l", uninstrumented, instrumented})
	if want := 0; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	// Only the uninstrumented file should be listed.
	if want, got := uninstrumented+"\n", out.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s\ndiff:\n%s", indent(got), indent(want), indent(diff.Lines(want, got)))
	}
}

func TestOptoutLines(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", `package foo
func _() {
	_ = "line 3" //errtrace:skip
	_ = "this line not counted" // errtrace:skip
	_ = "line 5" //errtrace:skip // has a reason
	_ = "line 6" //nolint:somelinter //errtrace:skip // stuff
}`, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var got []int
	for line := range optoutLines(fset, f.Comments) {
		got = append(got, line)
	}
	sort.Ints(got)

	if want := []int{3, 5, 6}; !reflect.DeepEqual(want, got) {
		t.Errorf("got: %v\nwant: %v\ndiff:\n%s", got, want, diff.Diff(want, got))
	}
}

func TestExpandPatterns(t *testing.T) {
	dir := t.TempDir()

	// Temporary directories on macOS are symlinked to /private/var/folders/...
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"go.mod":                         "module example.com/foo\n",
		"top.go":                         "package foo\n",
		"top_test.go":                    "package foo\n",
		"sub/sub.go":                     "package sub\n",
		"sub/sub_test.go":                "package sub\n",
		"sub/sub_ext_test.go":            "package sub_test\n",
		"testdata/ignored_by_default.go": "package testdata\n",
		"tagged.go":                      "//go:build mytag\npackage foo\n",
		"tagged_test.go":                 "//go:build mytag\npackage foo\n",
		"optout.go":                      "//go:build !errtrace\npackage foo\n",
	}

	for name, src := range files {
		dst := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(dst, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		tags []string
		args []string
		want []string
	}{
		{
			name: "stdin",
			args: []string{"-"},
			want: []string{"-"},
		},
		{
			name: "all",
			args: []string{"./..."},
			want: []string{
				"top.go",
				"top_test.go",
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
			},
		},
		{
			name: "all with tags",
			tags: []string{"mytag"},
			args: []string{"./..."},
			want: []string{
				"top.go",
				"top_test.go",
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
				"tagged.go",
				"tagged_test.go",
			},
		},
		{
			name: "relative subpackage",
			args: []string{"./sub"},
			want: []string{
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
			},
		},
		{
			name: "absolute subpackage",
			args: []string{"example.com/foo/sub/..."},
			want: []string{
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
			},
		},
		{
			name: "relative file",
			args: []string{"./sub/sub.go"},
			want: []string{
				"sub/sub.go",
			},
		},
		{
			name: "file and pattern",
			args: []string{
				"testdata/ignored_by_default.go",
				"./sub/...",
			},
			want: []string{
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
				"testdata/ignored_by_default.go",
			},
		},
		{
			name: "file and pattern with tags",
			tags: []string{"mytag"},
			args: []string{"./...", "testdata/ignored_by_default.go"},
			want: []string{
				"top.go",
				"top_test.go",
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
				"tagged.go",
				"tagged_test.go",
				"testdata/ignored_by_default.go",
			},
		},
		{
			name: "include opt-out explicitly",
			args: []string{"./...", "optout.go"},
			want: []string{
				"top.go",
				"top_test.go",
				"sub/sub.go",
				"sub/sub_test.go",
				"sub/sub_ext_test.go",
				"optout.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdir(t, dir)

			got, err := expandPatterns(tt.tags, tt.args)
			if err != nil {
				t.Fatal(err)
			}

			for i, p := range got {
				if filepath.IsAbs(p) {
					p, err = filepath.Rel(dir, p)
					if err != nil {
						t.Fatal(err)
					}
				}

				// Normalize slashes for cross-platform tests.
				got[i] = path.Clean(filepath.ToSlash(p))
			}

			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("got: %v\nwant: %v\ndiff:\n%s", got, tt.want, diff.Diff(tt.want, got))
			}
		})
	}
}

func TestExpandPatternsErrors(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := (&mainCmd{
		Stderr: &stderr,
		Stdout: testWriter{t},
	}).Run([]string{"["})
	if want := 1; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	if want, got := "malformed import path", stderr.String(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestGoListFilesCommandError(t *testing.T) {
	defer func(oldExecCommand func(string, ...string) *exec.Cmd) {
		_execCommand = oldExecCommand
	}(_execCommand)
	_execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("false")
	}

	var stderr bytes.Buffer
	exitCode := (&mainCmd{
		Stderr: &stderr,
		Stdout: testWriter{t},
	}).Run([]string{"./..."})
	if want := 1; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	if want, got := "go list: exit status 1", stderr.String(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestGoListFilesBadJSON(t *testing.T) {
	defer func(oldExecCommand func(string, ...string) *exec.Cmd) {
		_execCommand = oldExecCommand
	}(_execCommand)
	_execCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("echo", "bad json")
	}

	var stderr bytes.Buffer
	exitCode := (&mainCmd{
		Stderr: &stderr,
		Stdout: testWriter{t},
	}).Run([]string{"./..."})
	if want := 1; exitCode != want {
		t.Errorf("exit code = %d, want %d", exitCode, want)
	}

	if want, got := "go list: decode", stderr.String(); !strings.Contains(got, want) {
		t.Errorf("stderr = %q, want %q", got, want)
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
			_, lit, ok := strings.Cut(l.Text, "// want:")
			if !ok {
				continue
			}

			pos := fset.Position(l.Pos())
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
		if len(parts) == 4 {
			if _, err := strconv.Atoi(parts[2]); err == nil {
				// file:line:column:msg
				msg = parts[3]
			}
		}
		if msg == "" && len(parts) >= 2 {
			// file:line:msg
			msg = strings.Join(parts[2:], ":")
		}
		if msg == "" {
			return nil, fmt.Errorf("bad log line: %q", line)
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

func chdir(t testing.TB, dir string) (restore func()) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	var once sync.Once
	restore = func() {
		once.Do(func() {
			if err := os.Chdir(cwd); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Cleanup(restore)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return restore
}

type testWriter struct{ T testing.TB }

func (w testWriter) Write(p []byte) (int, error) {
	for _, line := range bytes.Split(p, []byte{'\n'}) {
		w.T.Logf("%s", line)
	}
	return len(p), nil
}
