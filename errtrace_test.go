package errtrace_test

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"braces.dev/errtrace"
)

func TestWrappedError(t *testing.T) {
	orig := errors.New("foo")
	err := errtrace.Wrap(orig)

	if want, got := "foo", err.Error(); want != got {
		t.Errorf("Error(): want %q, got %q", want, got)
	}
}

func TestWrappedErrorIs(t *testing.T) {
	orig := errors.New("foo")
	err := errtrace.Wrap(orig)

	if !errors.Is(err, orig) {
		t.Errorf("Is(): want true, got false")
	}
}

type myError struct{ x int }

func (m *myError) Error() string {
	return "great sadness"
}

func TestWrappedErrorAs(t *testing.T) {
	err := errtrace.Wrap(&myError{x: 42})
	var m *myError
	if !errors.As(err, &m) {
		t.Errorf("As(): want true, got false")
	}

	if want, got := 42, m.x; want != got {
		t.Errorf("As(): want %d, got %d", want, got)
	}
}

func TestFormatTrace(t *testing.T) {
	orig := errors.New("foo")

	f := func() error {
		return errtrace.Wrap(orig)
	}
	g := func() error {
		return errtrace.Wrap(f())
	}

	var h func(int) error
	h = func(n int) error {
		for n > 0 {
			return errtrace.Wrap(h(n - 1))
		}
		return errtrace.Wrap(g())
	}

	err := h(3)

	trace := errtrace.FormatString(err)

	// Line numbers change,
	// so verify function names and that the file name is correct.
	if want := "errtrace_test.go:"; !strings.Contains(trace, want) {
		t.Errorf("FormatString(): want trace to contain %q, got:\n%s", want, trace)
	}

	funcName := func(fn interface{}) string {
		return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	}

	if fName := funcName(f) + "\n"; !strings.Contains(trace, fName) {
		t.Errorf("FormatString(): want trace to contain %q, got:\n%s", fName, trace)
	}

	if gName := funcName(g) + "\n"; !strings.Contains(trace, gName) {
		t.Errorf("FormatString(): want trace to contain %q, got:\n%s", gName, trace)
	}

	hName := funcName(h) + "\n"
	if want, got := 4, strings.Count(trace, hName); want != got {
		t.Errorf("FormatString(): want trace to contain %d instances of %q, got %d\n%s", want, hName, got, trace)
	}
}

func BenchmarkWrap(b *testing.B) {
	err := errors.New("foo")
	for i := 0; i < b.N; i++ {
		_ = errtrace.Wrap(err)
	}
}

func BenchmarkFmtErrorf(b *testing.B) {
	err := errors.New("foo")
	for i := 0; i < b.N; i++ {
		_ = fmt.Errorf("bar: %w", err)
	}
}
