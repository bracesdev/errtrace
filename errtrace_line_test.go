package errtrace_test

import (
	_ "embed"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
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
	failed := errors.New("failed")

	tests := []struct {
		name string
		f    func() error
	}{
		{
			name: "return Wrap", // @group
			f: func() error {
				return errtrace.Wrap(failed) // @trace
			},
		},
		{
			name: "Wrap to intermediate and return", // @group
			f: func() (retErr error) {
				wrapped := errtrace.Wrap(failed) // @trace
				return wrapped
			},
		},
		{
			name: "Decorate error after Wrap", // @group
			f: func() (retErr error) {
				wrapped := errtrace.Wrap(failed) // @trace
				return fmt.Errorf("got err: %w", wrapped)
			},
		},
		{
			name: "defer updates errTrace", // @group
			f: func() (retErr error) {
				defer func() {
					retErr = errtrace.Wrap(retErr) // @trace
				}()

				return failed
			},
		},

		// Sanity testing for WrapN functions.
		{
			name: "Test Wrap2", // @group
			f: func() (retErr error) {
				nested := func() (int, error) {
					return errtrace.Wrap2(returnErr2()) // @trace
				}

				_, err := nested()
				return err
			},
		},
		{
			name: "Test Wrap3", // @group
			f: func() (retErr error) {
				nested := func() (int, int, error) {
					return errtrace.Wrap3(returnErr3()) // @trace
				}

				_, _, err := nested()
				return err
			},
		},
		{
			name: "Test Wrap4", // @group
			f: func() (retErr error) {
				nested := func() (int, int, int, error) {
					return errtrace.Wrap4(returnErr4()) // @trace
				}

				_, _, _, err := nested()
				return err
			},
		},
		{
			name: "Test Wrap5", // @group
			f: func() (retErr error) {
				nested := func() (int, int, int, int, error) {
					return errtrace.Wrap5(returnErr5()) // @trace
				}

				_, _, _, _, err := nested()
				return err
			},
		},
		{
			name: "Test Wrap6", // @group
			f: func() (retErr error) {
				nested := func() (int, int, int, int, int, error) {
					return errtrace.Wrap6(returnErr6()) // @trace
				}

				_, _, _, _, _, err := nested()
				return err
			},
		},
	}

	testMarkers, err := parseMarkers(errtraceLineTestFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed markers: %v", testMarkers)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := testMarkers[tt.name]
			if want, got := 1, len(markers); want != got {
				t.Fatalf("expected %v markers, got %v: %v", want, got, markers)
			}

			wantLine := markers[0]
			gotErr := tt.f()
			got := errtrace.FormatString(gotErr)
			wantFileLine := fmt.Sprintf("errtrace_line_test.go:%v", wantLine)
			if !strings.Contains(got, wantFileLine) {
				t.Errorf("formatted output is missing file:line %q in:\n%s", wantFileLine, got)
			}
		})
	}
}

func returnErr2() (int, error)                     { return 1, errors.New("test") }
func returnErr3() (int, int, error)                { return 1, 2, errors.New("test") }
func returnErr4() (int, int, int, error)           { return 1, 2, 3, errors.New("test") }
func returnErr5() (int, int, int, int, error)      { return 1, 2, 3, 4, errors.New("test") }
func returnErr6() (int, int, int, int, int, error) { return 1, 2, 3, 4, 5, errors.New("test") }

// parseMarkers parses the source file and returns a map
// from marker group name to line numbers in that group.
//
// Marker groups are identified by a '@group' comment
// immediately following a string literal -- ignoring operators.
// For example:
//
//	{
//		name: "foo", // @group
//		// Note that the "," is ignored as it's an operator.
//	}
//
// Markers in the group are identified by a '@trace' comment.
// For example:
//
//	{
//		name: "foo", // @group
//		f: func() error {
//			return errtrace.Wrap(failed) // @trace
//		},
//	}
//
// A group ends when a new group starts or the end of the file is reached.
func parseMarkers(src string) (map[string][]int, error) {
	// We don't need to parse the Go AST.
	// Just lexical analysis is enough.
	fset := token.NewFileSet()
	file := fset.AddFile("errtrace_line_test.go", fset.Base(), len(src))

	var (
		errs []error
		scan scanner.Scanner
	)
	scan.Init(
		file,
		[]byte(src),
		func(pos token.Position, msg string) {
			// This function is called for each error encountered
			// while scanning.
			errs = append(errs, fmt.Errorf("%v:%v", pos, msg))
		},
		scanner.ScanComments,
	)

	errf := func(pos token.Pos, format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		errs = append(errs, fmt.Errorf("%v:%v", file.Position(pos), msg))
	}

	markers := make(map[string][]int)
	var (
		currentMarker     string
		lastStringLiteral string
	)
	for {
		pos, tok, lit := scan.Scan()

		switch tok {
		case token.EOF:
			return markers, errors.Join(errs...)

		case token.STRING:
			s, err := strconv.Unquote(lit)
			if err != nil {
				errf(pos, "bad string literal: %v", err)
				continue
			}
			lastStringLiteral = s

		case token.COMMENT:
			switch lit {
			case "// @group":
				if lastStringLiteral == "" {
					errf(pos, "expected string literal before @group")
					continue
				}

				currentMarker = lastStringLiteral

			case "// @trace":
				if currentMarker == "" {
					errf(pos, "expected @group before @trace")
					continue
				}

				markers[currentMarker] = append(markers[currentMarker], file.Line(pos))
			}

		default:
			// For all other non-operator tokens, reset the last string literal.
			if !tok.IsOperator() {
				lastStringLiteral = ""
			}
		}
	}
}
