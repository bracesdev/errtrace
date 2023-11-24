# Automatic instrumentation

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
