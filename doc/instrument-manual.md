# Manual instrumentation

```go
import "braces.dev/errtrace"
```

Under manual instrumentation,
you're expected to import errtrace,
and wrap errors at all return sites like so:

```go
// ...
if err != nil {
    return errtrace.Wrap(err)
}
```

<details>
<summary>Example</summary>

Given a function like the following:

```go
func writeToFile(path string, src io.Reader) error {
  dst, err := os.Create(path)
  if err != nil {
    return err
  }
  defer dst.Close()

  if _, err := io.Copy(dst, src); err != nil {
    return err
  }

  return nil
}
```

With errtrace, you'd change it to:

```go
func writeToFile(path string, src io.Reader) error {
  dst, err := os.Create(path)
  if err != nil {
    return errtrace.Wrap(err)
  }
  defer dst.Close()

  if _, err := io.Copy(dst, src); err != nil {
    return errtrace.Wrap(err)
  }

  return nil
}
```

</details>

It's important that the `errtrace.Wrap` function is called
inside the same function that's actually returning the error.
A helper function will not suffice.


