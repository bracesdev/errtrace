# Error wrapping

errtrace operates by wrapping your errors to add caller information.

## Matching errors

As a result of errtrace's error wrapping,
wrapped errors cannot be matched directly with `==`.

```go
fmt.Println(err == err)                // true
fmt.Println(errtrace.Wrap(err) == err) // false
```

To match these wrapped errors, use the standard library's
[errors.Is](https://pkg.go.dev/errors#Is) function.

```go
fmt.Println(errors.Is(errtrace.Wrap(err), err)) // true
```

## Casting errors

Similarly, as a result of errtrace's error wrapping,
wrapped errors cannot be type-cast directly.

```go
my, ok := err.(*MyError)                // ok = true
my, ok := errtrace.Wrap(err).(*MyError) // ok = false
```

To type-cast wrapped errors, use the standard library's
[errors.As](https://pkg.go.dev/errors#As) function.

```go
var my *MyError
ok := errors.As(errtrace.Wrap(err), &my) // ok = true
```

## Linting

You can use [go-errorlint](https://github.com/polyfloyd/go-errorlint)
to find places in your code
where you're comparing errors with `==` instead of using `errors.Is`
or type-casting them directly instead of using `errors.As`.
