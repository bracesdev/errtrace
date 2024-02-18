package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"braces.dev/errtrace"
)

func (cmd *mainCmd) handleToolExec(args []string) (exitCode int, handled bool) {
	// In toolexec mode, we're passed the original command + arguments.
	if len(args) == 0 {
		return -1, false
	}

	for _, arg := range args {
		if arg == "-V=full" {
			// compile is run first with "-V=full" to get a version number
			// for caching build IDs.
			// No TOOLEXEC_IMPORTPATH is set in this case.
			return cmd.toolExecVersion(args), true
		}
	}

	if cmd.Getenv == nil {
		cmd.Getenv = os.Getenv
	}
	// When "-toolexec" is used, the go cmd sets the package being compiled in the env.
	if pkg := cmd.Getenv("TOOLEXEC_IMPORTPATH"); pkg != "" {
		return cmd.toolExecRewrite(pkg, args), true
	}

	return -1, false
}

func (cmd *mainCmd) toolExecVersion(args []string) int {
	tool := exec.Command(args[0], args[1:]...)
	var stdout bytes.Buffer
	tool.Stdout = &stdout
	tool.Stderr = cmd.Stderr
	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}

		fmt.Fprintf(cmd.Stderr, "%v failed: %v", args[0], err)
		return 1
	}

	// TODO: This version number should change whenever the rewriting changes.
	fmt.Fprintf(cmd.Stdout, "%s-errtrace0\n", strings.TrimSpace(stdout.String()))
	return 0
}

func (cmd *mainCmd) toolExecRewrite(pkg string, args []string) (exitCode int) {
	// We only need to modify the arguments for "compile" calls which work with .go files.
	if !isCompile(args[0]) {
		return cmd.runOriginal(args)
	}

	// We only modify files that import errtrace, so stdlib is never eliglble.
	// To avoid unnecessary parsing, use a heuristic to detect stdlib packages --
	// whether the name contains ".".
	if !strings.Contains(pkg, ".") {
		return cmd.runOriginal(args)
	}

	exitCode, err := cmd.rewriteCompile(pkg, args)
	if err != nil {
		cmd.log.Print(err)
		return 1
	}

	return exitCode
}

func (cmd *mainCmd) rewriteCompile(pkg string, args []string) (exitCode int, _ error) {
	parsed := make(map[string]parsedFile)
	var canRewrite, needRewrite bool
	for _, arg := range args {
		if !isGoFile(arg) {
			continue
		}

		contents, err := os.ReadFile(arg)
		if err != nil {
			return -1, errtrace.Wrap(err)
		}

		f, err := cmd.parseFile(arg, contents)
		if err != nil {
			return -1, errtrace.Wrap(err)
		}
		parsed[arg] = f

		// TODO: Support an "unsafe" mode to rewrite packages without errtrace imports.
		if f.importsErrtrace {
			canRewrite = true
		}
		if len(f.inserts) > 0 {
			needRewrite = true
		}
	}

	if !canRewrite || !needRewrite {
		return cmd.runOriginal(args), nil
	}

	// Use a temporary directory per-package that is rewritten.
	tempDir, err := os.MkdirTemp("", filepath.Base(pkg))
	if err != nil {
		return -1, errtrace.Wrap(err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // best-effort removal of temp files.

	newArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if !isGoFile(arg) {
			newArgs = append(newArgs, arg)
			continue
		}

		f, ok := parsed[arg]
		if !ok || len(f.inserts) == 0 {
			newArgs = append(newArgs, arg)
			continue
		}

		// Add a //line directive so the original filepath is used in errors and panics.
		var out bytes.Buffer
		_, _ = fmt.Fprintf(&out, "//line %v:1\n", arg)

		if err := cmd.rewriteFile(f, &out); err != nil {
			return -1, errtrace.Wrap(err)
		}

		newFile := filepath.Join(tempDir, filepath.Base(arg))
		if err := os.WriteFile(newFile, out.Bytes(), 0o666); err != nil {
			return -1, errtrace.Wrap(err)
		}

		newArgs = append(newArgs, newFile)
	}

	return cmd.runOriginal(newArgs), nil
}

func isCompile(arg string) bool {
	if runtime.GOOS == "windows" {
		arg = strings.TrimSuffix(arg, ".exe")
	}
	return strings.HasSuffix(arg, "compile")
}

func isGoFile(arg string) bool {
	return strings.HasSuffix(arg, ".go")
}

func (cmd *mainCmd) runOriginal(args []string) (exitCode int) {
	tool := exec.Command(args[0], args[1:]...)
	tool.Stdin = cmd.Stdin
	tool.Stdout = cmd.Stdout
	tool.Stderr = cmd.Stderr

	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(cmd.Stderr, "tool failed: %v", err)
		return 1
	}

	return 0
}
