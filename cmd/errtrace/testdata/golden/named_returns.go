//go:build ignore

package foo

import (
	"errors"
	"fmt"
)

func NakedReturn(s string) (err error) {
	err = errors.New("sadness: " + s)
	fmt.Println("Reporting sadness")
	return
}

func NamedReturn(s string) (err error) {
	err = errors.New("sadness: " + s)
	fmt.Println("Reporting sadness")
	return err
}

func MultipleErrors() (err1, err2 error, ok bool, err3, err4 error) {
	err1 = errors.New("a")
	err2 = errors.New("b")
	ok = false
	err3 = errors.New("c")
	err4 = errors.New("d")

	if !ok {
		// Naked
		return
	}

	// Named
	return err1, err2, ok, err3, err4
}

func UnderscoreNamed() (_ error) {
	return NamedReturn("foo")
}

func UnderscoreNamedMultiple() (_ bool, err error) {
	return false, NamedReturn("foo")
}

func DeferWrapNamed() (err error) {
	defer func() {
		err = fmt.Errorf("wrapped: %w", err)
	}()

	return NamedReturn("foo")
}

func DeferWrapNamedWithItsOwnError() (_ int, err error) {
	// Both, the error returned by the deferred function
	// and the named error wrapped by it should be wrapped.
	defer func() error {
		err = fmt.Errorf("wrapped: %w", err)

		return errors.New("ignored")
	}()

	return 0, UnderscoreNamed()
}
