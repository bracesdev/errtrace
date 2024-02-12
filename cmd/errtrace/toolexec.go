package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cmd *mainCmd) RunToolExec(toolExecPkg string, args []string) (exitCode int) {
	args = args[1:]
	if len(args) == 0 {
		fmt.Fprintf(cmd.Stderr, "toolexec expected command to run + args")
		return 1
	}

	// replace the .go file with a errtraced version
	if strings.HasSuffix(args[0], "compile") && strings.Contains(toolExecPkg, "braces.dev") && strings.Contains(toolExecPkg, "foo") {
		tempDir, err := os.MkdirTemp("", filepath.Base(toolExecPkg))
		if err != nil {
			panic(err)
		}

		for i, arg := range args {
			if strings.HasSuffix(arg, ".go") {
				fmt.Fprintf(cmd.Stderr, "change file in pkg: %v, %v\n", toolExecPkg, arg)
				filename := filepath.Join(tempDir, filepath.Base(arg))
				req := fileRequest{
					ReadFile: arg,
					Filename: filename,
					Write:    true,
				}
				if err := cmd.processFile(req); err != nil {
					panic(fmt.Sprintf("%+v", err))
				}
				args[i] = filename
			}
		}
	}

	tool := exec.Command(args[0], args[1:]...)
	tool.Stdin = cmd.Stdin
	tool.Stdout = cmd.Stdout
	tool.Stderr = cmd.Stderr
	tool.Env = os.Environ()

	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		} else {
			fmt.Fprintf(cmd.Stderr, "tool failed: %v", err)
			return 1
		}
	}

	if toolExecPkg == "" {
		fmt.Fprintf(cmd.Stdout, "errtrace-v0")
	}

	return 0
}

func (cmd *mainCmd) toolExecVersion(args []string) int {
	args = args[1:]
	if len(args) == 0 {
		fmt.Fprintf(cmd.Stderr, "toolexec expected command to run + args")
		return 1
	}

	var stdout bytes.Buffer
	tool := exec.Command(args[0], args[1:]...)
	tool.Stdout = &stdout
	tool.Stderr = cmd.Stderr
	tool.Env = os.Environ()
	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		} else {
			fmt.Fprintf(cmd.Stderr, "tool failed: %v", err)
			return 1
		}
	}

	fmt.Fprintf(cmd.Stdout, "%s-errtrace0\n", strings.TrimSpace(stdout.String()))
	return 0
}

func isToolExec(args []string, getenv func(string) string) (string, bool) {
	for _, arg := range args {
		if arg == "-V=full" {
			return "", true
		}
	}

	pkg := getenv("TOOLEXEC_IMPORTPATH")
	return pkg, pkg != ""
}
