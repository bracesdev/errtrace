//go:build ignore

package foo

import (
	"errors"
	"fmt"
	"strconv"
)

func Int(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	return i + 42, nil
}

func NamedReturn_Naked(s string) (err error) {
	err = errors.New("sadness: " + s)
	fmt.Println("Reporting sadness")
	return
}

func HasFunctionLiteral() {
	err := func() error {
		return errors.New("sadness")
	}()

	fmt.Println(err)
}

// TODO: multiple return values without variables
