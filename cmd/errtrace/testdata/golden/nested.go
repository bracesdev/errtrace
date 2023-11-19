//go:build ignore

package foo

import (
	"errors"
	"fmt"
)

func HasFunctionLiteral() {
	err := func() error {
		return errors.New("sadness")
	}()

	fmt.Println(err)
}

func ImmediatelyInvokedFunctionExpression() error {
	return func() error {
		return errors.New("sadness")
	}()
}
