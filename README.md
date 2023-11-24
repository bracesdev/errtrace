# errtrace

- [Introduction](#introduction)
  - [Installation](#installation)
- [Usage](#usage)
  - [Manual](#manual-instrumentation)
  - [Automatic](#automatic-instrumentation)
- [Performance](#performance)
- [Caveats](#caveats)
  - [Error wrapping](#error-wrapping)
  - [Safety](#safety)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Introduction

<div align="center">
  <img src="doc/assets/logo.png" width="300"/>
</div>

> **Warning**:
> errtrace is extremely experimental.
> Use it at your own risk.

errtrace is an experimental package to trace an error's return path
through a Go program.

Rather than providing a stack trace
showing the *inwards* route that caused an error,
errtrace lets you track the *outwards* route that the error took
until you ultimately handle it.
We believe that this can be more useful than a plain stack trace
for complex programs written in Go.

### Features

- **Lightweight**:
  errtrace brings no other runtime dependencies with it.
- [**Simple**](#manual-instrumentation):
  The library API is simple, straightforward, and idiomatic.
- [**Automatic**](#automatic-instrumentation):
  The errtrace CLI will automatically instrument your code.
- [**Fast**](#performance):
  On popular 64-bit systems, errtrace is much faster
  than capturing a stack trace.

### Why

In languages like Go where errors are values,
users have the ability to store the error in a struct,
pass it through a channel, etc.

This can result in a situation where a stack trace,
which records the path *to the error*,
loses some usefulness as it moves through the program
before it's surfaced to the user.

We believe that for such programs,
it can be more useful and more performant
to have the return trace instead:
the path the error took *out* to get to the user.

This library is an experiment to evaluate that idea.

### Installation

Install errtrace with Go modules:

```bash
go get braces.dev/errtrace@latest
```

If you want to use the CLI, use `go install`.

```bash
go install braces.dev/errtrace/cmd/errtrace@latest
```

## Usage

errtrace offers the following modes of usage:

- [Manual instrumentation](#manual-instrumentation)
- [Automatic instrumentation](#automatic-instrumentation)

### Manual instrumentation

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

### Automatic instrumentation

If manual instrumentation is too much work (we agree),
we've included a tool that will automatically instrument
all your code with errtrace.

First, [install the tool](#installation), and then run it with one or more Go files:

```bash
errtrace -w path/to/file.go path/to/another/file.go
```

If you'd like to run it on all Go files in your project,
and you use Git, run the following on a Unix-like system:

```bash
git ls-files '*.go' | xargs errtrace -w
```

errtrace can be set be setup as a custom formatter in your editor,
similar to gofmt or goimports.

## Performance

errtrace is designed to have very low overhead
on supported systems.

Benchmark results for linux/amd64 on an Intel Core i5-13600 (best of 10):

```
BenchmarkFmtErrorf      11574928               103.5 ns/op            40 B/op          2 allocs/op
# default build, uses Go assembly.
BenchmarkWrap           78173496                14.70 ns/op           24 B/op          0 allocs/op
# build with -tags safe to avoid assembly.
BenchmarkWrap            5958579               198.5 ns/op            24 B/op          0 allocs/op

# benchext compares capturing stacks using pkg/errors vs errtrace
# both tests capture ~10 frames,
BenchmarkErrtrace        6388651               188.4 ns/op           280 B/op          1 allocs/op
BenchmarkPkgErrors       1673145               716.8 ns/op           304 B/op          3 allocs/op
```

Stack traces have a large initial cost,
while errtrace scales with each frame that an error is returned through.

## Caveats

### Error wrapping

errtrace operates by wrapping your errors to add caller information.

#### Matching errors

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

#### Casting errors

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

#### Linting

You can use [go-errorlint](https://github.com/polyfloyd/go-errorlint)
to find places in your code
where you're comparing errors with `==` instead of using `errors.Is`
or type-casting them directly instead of using `errors.As`.

### Safety

To achieve the performance above,
errtrace makes use of unsafe operations using Go assembly
to read the caller information directly from the stack.
This is part of the reason why we have the disclaimer on top.

errtrace includes an opt-in safe mode
that drops these unsafe operations in exchange for poorer performance.
To opt into safe mode,
use the `safe` build tag when compiling code that uses errtrace.

```bash
go build -tags safe
```

## Acknowledgements

The idea of tracing return paths instead of stack traces
comes from [Zig's error return traces](https://ziglang.org/documentation/0.11.0/#Error-Return-Traces).

## License

This software is made available under the BSD3 license.
See LICENSE file for details.
