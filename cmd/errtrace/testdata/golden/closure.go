//go:build ignore

package foo

import (
	"errors"
	"fmt"
)

func ClosureReturnsError() error {
	return func() error {
		return errors.New("great sadness")
	}()
}

func ClosureDoesNotReturnError() error {
	x := func() int {
		return 42
	}()
	return nil
}

func DeferedClosureReturnsError() error {
	defer func() error {
		// The error is ignored,
		// but it should still be wrapped.
		return errors.New("great sadness")
	}()

	return nil
}

func DeferedClosureDoesNotReturnError() error {
	defer func() int {
		return 42
	}()

	return nil
}

func ClosureReturningErrorHasDifferentNumberOfReturns() (int, error) {
	x := func() error {
		return errors.New("great sadness")
	}

	return 42, x()
}

func ClosureNotReturningErrorHasDifferentNumberOfReturns() (int, error) {
	x := func() int {
		return 42
	}

	return 42, nil
}

func ClosureInsideAnotherClosure() error {
	return func() error {
		return func() error {
			return errors.New("great sadness")
		}()
	}()
}

func ClosureNotReturningErrorInsideAnotherClosure() (int, error) {
	var x int
	err := func() error {
		x := func() int {
			return 42
		}()
		return errors.New("great sadness")
	}()

	return x, err
}

func ClosureReturningAnErrorInsideADefer() error {
	defer func() {
		err := func() error {
			return errors.New("great sadness")
		}()

		fmt.Println(err)
	}()

	return nil
}

func ClosureNotReturningAnErrorInsideADefer() error {
	defer func() error {
		x := func() int {
			return 42
		}()

		return fmt.Errorf("great sadness: %d", x)
	}()

	return nil
}
