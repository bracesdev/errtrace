# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- Add -l flag to cmd/errtrace.
  This prints files that would be changed without changing them.

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
