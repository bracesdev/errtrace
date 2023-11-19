//go:build ignore

package foo

type myError struct{}

func (*myError) Error() string {
	return "sadness"
}

func Try() error {
	return &myError{}
}
