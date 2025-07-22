package p3

import (
	"errors"
)

// ReturnStrErr returns an error.
func ReturnStrErr() (string, error) {
	return "", errors.New("test") // @trace
}
