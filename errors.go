package errtrace

import (
	"errors"
	"fmt"

	"braces.dev/errtrace/internal/pc"
)

// New returns an error with the supplied text.
//
// It's equivalent to [errors.New] followed by [Wrap] to add caller information.
//
//go:noinline due to GetCaller (see [Wrap] for details).
func New(text string) error {
	return wrap(errors.New(text), pc.GetCaller())
}

// Errorf creates an error message
// according to a format specifier
// and returns the string as a value that satisfies error.
//
// It's equivalent to [fmt.Errorf] followed by [Wrap] to add caller information.
//
//go:noinline due to GetCaller (see [Wrap] for details).
func Errorf(format string, args ...any) error {
	return wrap(fmt.Errorf(format, args...), pc.GetCaller())
}
