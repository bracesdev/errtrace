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

func TestWrapNil(t *testing.T) {
	if err := errtrace.Wrap(nil); err != nil {
		t.Errorf("Wrap(): want nil, got %v", err)
	}
}

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

func TestFormatVerbs(t *testing.T) {
	err := errors.New("error")
	wrapped := errtrace.Wrap(err)

	tests := []struct {
		name string
		fmt  string
		want string
	}{
		{
			name: "verb s",
			fmt:  "%s",
			want: "error",
		},
		{
			name: "verb v",
			fmt:  "%v",
			want: "error",
		},
		{
			name: "verb q",
			fmt:  "%q",
			want: `"error"`,
		},
		{
			name: "padded string",
			fmt:  "%10s",
			want: "     error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if want, got := tt.want, fmt.Sprintf(tt.fmt, err); want != got {
				t.Errorf("unwrapped: want %q, got %q", want, got)
			}

			if want, got := tt.want, fmt.Sprintf(tt.fmt, wrapped); want != got {
				t.Errorf("wrapped: want %q, got %q", want, got)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	e1 := errtrace.New("e1")
	e2 := errtrace.Errorf("e2: %w", e1)
	e3 := errtrace.Wrap(e2)

	tests := []struct {
		name       string
		err        error
		want       string
		wantTraces int
	}{
		{
			name:       "new error",
			err:        e1,
			want:       "e1",
			wantTraces: 1,
		},
		{
			name:       "wrapped with Errorf",
			err:        e2,
			want:       "e2: e1",
			wantTraces: 2,
		},
		{
			name:       "wrap after Errorf",
			err:        e3,
			want:       "e2: e1",
			wantTraces: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if want, got := tt.want, fmt.Sprintf("%s", tt.err); want != got {
				t.Errorf("message: want %q, got: %q", want, got)
			}

			withTrace := fmt.Sprintf("%+v", tt.err)
			if !strings.HasPrefix(withTrace, tt.want) {
				t.Errorf("expected error message %q in trace:\n%s", tt.want, withTrace)
			}
			if want, got := tt.wantTraces, strings.Count(withTrace, "errtrace_test.TestFormat"); want != got {
				t.Errorf("expected traces %v, got %v in:\n%s", want, got, withTrace)
			}
		})
	}
}

func BenchmarkWrap(b *testing.B) {
	err := errors.New("foo")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = errtrace.Wrap(err)
		}
	})
}

func BenchmarkFmtErrorf(b *testing.B) {
	err := errors.New("foo")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = fmt.Errorf("bar: %w", err)
		}
	})
}
