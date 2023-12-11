# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- cmd/errtrace: Support Go package patterns in addition to file paths.
  Use `errtrace -w ./...` to transform all files under the current package
  and its descendants.

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
