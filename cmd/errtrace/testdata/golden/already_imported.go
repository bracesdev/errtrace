//go:build ignore

package foo

import (
	"strconv"

	"braces.dev/errtrace"
)

func Unwrapped(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return i + 42, nil
}

func AlreadyWrapped(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, errtrace.Wrap(err)
	}
	return i + 42, nil
}

func SkipNew() error {
	return errtrace.New("test")
}

func SkipErrorf() error {
	return errtrace.Errorf("foo: %v", SkipNew())
}
