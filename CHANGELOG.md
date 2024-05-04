# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- Add UnwrapFrame function to extract a single frame from an error.
  You can use this to implement your own trace formatting logic.

### Fixed
- cmd/errtrace: Don't exit with a non-zero status when `-h` is used.
- cmd/errtrace: Don't panic on imbalanced assignments inside defer blocks.

## v0.3.0 - 2023-12-22

This release adds support to the CLI for using Go package patterns like `./...`
to match and transform files.
You can now use `errtrace -w ./...` to instrument all files in a Go module,
or `errtrace -l ./...` to list all files that would be changed.

### Added
- cmd/errtrace: Support Go package patterns in addition to file paths.
  Use `errtrace -w ./...` to transform all files under the current package
  and its descendants.

### Changed
- cmd/errtrace:
  Print a message when reading from stdin because no arguments were given.
  Use '-' as the file name to read from stdin without a warning.

## v0.2.0 - 2023-11-30

This release contains minor improvements to the errtrace code transformer
allowing it to fit more use cases.

### Added
- cmd/errtrace:
  Add -l flag to print files that would be changed without changing them.
  You can use this to build a check to verify that your code is instrumented.
- cmd/errtrace: Support opt-out on lines with a `//errtrace:skip` comment.
  Optionally, a reason may be specified alongside the comment.
  The command will print a warning for any unused `//errtrace:skip` comments.

  ```go
  if err != nil {
    return io.EOF //errtrace:skip(io.Reader expects io.EOF)
  }
  ```

## v0.1.1 - 2023-11-28
### Changed
- Lower `go` directive in go.mod to 1.20
  to allow use with older versions.

### Fixed
- Add a README.md to render alongside the
  [API reference](https://pkg.go.dev/braces.dev/errtrace).

## v0.1.0 - 2023-11-28

Introducing errtrace, an experimental library
that provides better stack traces for your errors.

Install the library with:

```bash
go get braces.dev/errtrace@v0.1.0
```

We've also included a tool
that will automatically instrument your code with errtrace.
In your project, run:

```bash
go install braces.dev/errtrace/cmd/errtrace@v0.1.0
git ls-files -- '*.go' | xargs errtrace -w
```

See [README](https://github.com/bracesdev/errtrace#readme)
for more information.
