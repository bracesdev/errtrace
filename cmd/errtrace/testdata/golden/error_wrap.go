//go:build ignore

package foo

type innerError struct{}

func (*innerError) Error() string {
	return "sadness"
}

type errorWrapper struct {
	Err error
}

func (e *errorWrapper) Error() string {
	return e.Err.Error()
}

// Unwrap should not be wrapped by errtrace.
func (e *errorWrapper) Unwrap() error {
	return e.Err
}

func Try() error {
	return &errorWrapper{Err: &innerError{}}
}
