package foo

import "errors"

func Foo() error {
	return errors.New("test")
}
