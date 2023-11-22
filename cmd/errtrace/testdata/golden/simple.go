//go:build ignore

package foo

import (
	"io"
	"os"
	"strconv"
)

func Unwrapped(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return i + 42, nil
}

func Parse(s string) (int, error) {
	return strconv.Atoi(s)
}

func DeferWithoutNamedReturns(s string) error {
	f, err := os.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	return nil
}
