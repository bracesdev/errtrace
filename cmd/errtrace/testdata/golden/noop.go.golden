//go:build ignore

package foo

import "errors"

// This file should not be changed.

func success() error {
	return nil
}

func failure() error {
	return errors.New("failure") //errtrace:skip
}

func deferred() (err error) {
	defer func() {
		err = errors.New("failure") //errtrace:skip
	}()
	return nil
}

func namedReturn() (err error) {
	err = errors.New("failure")
	return //errtrace:skip
}

func immediatelyInvoked() error {
	return func() error { //errtrace:skip
		return errors.New("failure") //errtrace:skip
	}()
}

func multipleReturns() (error, error) {
	return errors.New("a"), errors.New("b") //errtrace:skip
}
