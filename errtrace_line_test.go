package errtrace_test

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"braces.dev/errtrace"
)

//go:embed errtrace_line_test.go
var errtraceLineTestFile string

// Note: The following tests verify the line, and assume that the
// test names are unique, and that they are the only tests in this file.
func TestWrap_Line(t *testing.T) {
	var failed = errors.New("failed")
	tests := []struct {
		name string
		f    func() error
	}{
		{
			name: "return Wrap",
			f: func() error {
				return errtrace.Wrap(failed) // trace line
			},
		},
		{
			name: "Wrap to intermediate and return",
			f: func() (retErr error) {
				wrapped := errtrace.Wrap(failed) // trace line
				return wrapped
			},
		},
		{
			name: "Decorate error after Wrap",
			f: func() (retErr error) {
				wrapped := errtrace.Wrap(failed) // trace line
				return fmt.Errorf("got err: %w", wrapped)
			},
		},
		{
			name: "defer updates errTrace",
			f: func() (retErr error) {
				defer func() {
					retErr = errtrace.Wrap(retErr) // trace line
				}()

				return failed
			},
		},
	}

	wantLineNumbers := parseTestWants()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantLine, ok := wantLineNumbers[tt.name]
			if !ok {
				t.Fatalf("failed to find `// trace line` for test %q", tt.name)
			}

			gotErr := tt.f()
			got := errtrace.FormatString(gotErr)
			wantFileLine := fmt.Sprintf("errtrace_line_test.go:%v", wantLine)
			if !strings.Contains(got, wantFileLine) {
				t.Errorf("formatted output is missing file:line %q in:\n%s", wantFileLine, got)
			}
		})
	}
}

func parseTestWants() map[string]int {
	lines := strings.Split(errtraceLineTestFile, "\n")

	testWants := make(map[string]int)
	var lastTestName string
	for i, line := range lines {
		if name, ok := strings.CutPrefix(line, "\t\t\tname: "); ok {
			// trim and unquote name which looks like `"foo",`
			name = strings.TrimSpace(strings.TrimSuffix(name, ","))
			unquoted, err := strconv.Unquote(name)
			if err != nil {
				fmt.Println(err)
				panic(fmt.Sprintf("expected test name to be quoted, got %q", name))
			}

			lastTestName = unquoted
			continue
		}

		if strings.Contains(line, "// trace line") {
			testWants[lastTestName] = i + 1 // indexes start 0, lines start at 1.
		}

		if strings.Contains(line, "parseTestWants") {
			break
		}
	}

	return testWants
}
