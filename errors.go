package errtrace

import (
	"errors"
	"fmt"

	"braces.dev/errtrace/internal/pc"
)

// New returns an error that formats as the given text, similar to `errors.New`.
//
// It adds informtaion about the program counter of the caller to the error, see
// `Wrap` for details.
//
// This helper is a simpler alternative to `errtrace.Wrap(errors.New(...))`.
func New(text string) error {
	return wrap(errors.New(text), pc.GetCaller())
}

// Errorf formats according to a format specifier and returns the string as a
// value that satisfies error, similar to `fmt.Errorf`.
//
// It adds informtaion about the program counter of the caller to the error, see
// `Wrap` for details.
//
// This helper is a simpler alternative to `errtrace.Wrap(fmt.Errorf(...))`.
func Errorf(format string, args ...any) error {
	return wrap(fmt.Errorf(format, args...), pc.GetCaller())
}
