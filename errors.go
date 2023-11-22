package errtrace

import (
	"errors"
	"fmt"

	"braces.dev/errtrace/internal/pc"
)

// New returns an error that formats as the given text. It's equivalent to
// [errors.New] followed by [Wrap] to add caller information.
func New(text string) error {
	return wrap(errors.New(text), pc.GetCaller())
}

// Errorf formats according to a format specifier and returns the string as a
// value that satisfies error. It's equivalent to [fmt.Errorf] followed by
// [Wrap] to add caller information.
func Errorf(format string, args ...any) error {
	return wrap(fmt.Errorf(format, args...), pc.GetCaller())
}
