package p3

import (
	"errors"
)

// ReturnErr returns an error.
func ReturnErr() error {
	return errors.New("test") // @trace
}
