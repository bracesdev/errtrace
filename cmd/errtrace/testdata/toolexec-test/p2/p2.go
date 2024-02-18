package p2

import (
	"errors"

	"braces.dev/errtrace"
)

// ReturnErr returns an error.
func ReturnErr() error {
	return errtrace.Wrap(errors.New("test")) // @trace
}
