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
