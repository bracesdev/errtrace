package errtrace_test

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"braces.dev/errtrace"
)

var safe = false

// Note: Though test tables could DRY up the below tests, they're intentionally
// kept as functions calling other simple functions to test how inlining impacts
// use of `GetCaller`.

func TestGetCallerWrap_ErrorsNew(t *testing.T) {
	err := callErrorsNew()
	wantErr(t, err, "callErrorsNew")
}

func callErrorsNew() error {
	return errorsNew("wrap errors.New")
}

func errorsNew(msg string) error {
	caller := errtrace.GetCaller()
	return caller.Wrap(errors.New(msg))
}

func TestGetCallerWrap_WrapExisting(t *testing.T) {
	err := callWrapExisting()
	wantErr(t, err, "callWrapExisting")
}

func callWrapExisting() error {
	return wrapExisting()
}

var errFoo = errors.New("foo")

func wrapExisting() error {
	return errtrace.GetCaller().Wrap(errFoo)
}

func TestGetCallerWrap_PassCaller(t *testing.T) {
	err := callPassCaller()
	wantErr(t, err, "callPassCaller")
}

func callPassCaller() error {
	return passCaller()
}

func passCaller() error {
	return passCallerInner(errtrace.GetCaller())
}

func passCallerInner(caller errtrace.Caller) error {
	return caller.Wrap(errFoo)
}

func TestGetCallerWrap_RetCaller(t *testing.T) {
	err := callRetCaller()

	wantFn := "callRetCaller"
	if !safe {
		// If the function calling pc.GetCaller is inlined, there's no stack frame
		// so the assembly implementation can skip the correct caller.
		// Callers of GetCaller using `go:noinline` avoid this (as recommended in the docs).
		// Inlining is not consistent, hence we check the frame in !safe mode.
		f, _, _ := errtrace.UnwrapFrame(err)
		if !strings.HasSuffix(f.Function, wantFn) {
			wantFn = "TestGetCallerWrap_RetCaller"
		}
	}
	wantErr(t, err, wantFn)
}

func callRetCaller() error {
	return retCaller().Wrap(errFoo)
}

func retCaller() errtrace.Caller {
	return errtrace.GetCaller()
}

func TestGetCallerWrap_RetCallerNoInline(t *testing.T) {
	err := callRetCallerNoInline()
	wantErr(t, err, "callRetCallerNoInline")
}

func callRetCallerNoInline() error {
	return retCallerNoInline().Wrap(errFoo)
}

//go:noinline
func retCallerNoInline() errtrace.Caller {
	return errtrace.GetCaller()
}

func wantErr(t testing.TB, err error, fn string) runtime.Frame {
	if err == nil {
		t.Fatalf("expected err")
	}

	f, _, _ := errtrace.UnwrapFrame(err)
	if !strings.HasSuffix(f.Function, "."+fn) {
		t.Errorf("expected caller to be %v, got %v", fn, f.Function)
	}
	return f
}
