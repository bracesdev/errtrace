//go:build ignore

package foo

import (
	"errors"
	"io"

	"example.com/bar"
)

func Try(problem bool) (int, error) {
	err := bar.Do(func() error {
		if problem {
			return errors.New("great sadness")
		}

		return io.EOF //errtrace:skip // expects io.EOF
	})
	if err != nil {
		return 0, err
	}

	return bar.Baz() //errtrace:skip // caller wants unwrapped error
}

func unused() error {
	return nil //errtrace:skip // want:"unused errtrace:skip"
}

func multipleReturns() (a, b error) {
	return errors.New("a"),
		errors.New("b") //errtrace:skip
}
