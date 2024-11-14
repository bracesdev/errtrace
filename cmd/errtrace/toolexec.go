package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"braces.dev/errtrace"
)

func (cmd *mainCmd) handleToolExec(args []string) (exitCode int, handled bool) {
	// In toolexec mode, we're passed the original command + arguments.
	if len(args) == 0 {
		return -1, false
	}

	if cmd.Getenv == nil {
		cmd.Getenv = os.Getenv
	}

	// compile is run first with "-V=full" to get a version number
	// for caching build IDs.
	// No TOOLEXEC_IMPORTPATH is set in this case.
	version := slices.Contains(args, "-V=full")
	pkg := cmd.Getenv("TOOLEXEC_IMPORTPATH")
	if !version && pkg == "" {
		return -1, false
	}

	var p toolExecParams
	if err := p.Parse(os.Stdout, args); err != nil {
		cmd.log.Print(err)
		return 1, true
	}

	if version {
		return cmd.toolExecVersion(p), true
	}
	return cmd.toolExecRewrite(pkg, p), true
}

type toolExecParams struct {
	RequiredPkgSelectors []string

	Tool     string
	ToolArgs []string

	flags *flag.FlagSet
}

func (p *toolExecParams) Parse(w io.Writer, args []string) error {
	p.flags = flag.NewFlagSet("errtrace (toolexec)", flag.ContinueOnError)
	flag.Usage = func() {
		logln(w, `usage with go build/run/test: -toolexec="errtrace [options]"`)
		flag.PrintDefaults()
	}
	var requiredPkgs string
	p.flags.StringVar(&requiredPkgs, "required-packages", "", "comma-separated list of package selectors "+
		"that are expected to be import errtrace if they return errors.")

	// Flag parsing stops at the first non-flag argument (no "-").
	if err := p.flags.Parse(args); err != nil {
		return errtrace.Wrap(err)
	}

	remArgs := p.flags.Args()
	if len(remArgs) == 0 {
		return errtrace.New("toolexec expected tool arguments")
	}

	p.Tool = remArgs[0]
	p.ToolArgs = remArgs[1:]
	p.RequiredPkgSelectors = strings.Split(requiredPkgs, ",")
	return nil
}

// Options affect the generated code, so use a hash
// of any options for the toolexec version.
func (p *toolExecParams) versionCacheKey() string {
	withoutTool := *p
	withoutTool.flags = nil
	withoutTool.Tool = ""
	withoutTool.ToolArgs = nil

	optStr := fmt.Sprintf("%v", withoutTool)
	optHash := md5.Sum([]byte(optStr))
	return hex.EncodeToString(optHash[:])
}

func (p *toolExecParams) requiredPackage(pkg string) bool {
	for _, selector := range p.RequiredPkgSelectors {
		if packageSelectorMatch(selector, pkg) {
			return true
		}
	}
	return false
}

func (cmd *mainCmd) toolExecVersion(p toolExecParams) int {
	version, err := binaryVersion()
	if err != nil {
		logf(cmd.Stderr, "errtrace version failed: %v", err)
	}

	tool := exec.Command(p.Tool, p.ToolArgs...)
	var stdout bytes.Buffer
	tool.Stdout = &stdout
	tool.Stderr = cmd.Stderr
	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}

		logf(cmd.Stderr, "tool %v failed: %v", p.Tool, err)
		return 1
	}

	if _, err := fmt.Fprintf(
		cmd.Stdout,
		"%s-errtrace-%s%s\n",
		strings.TrimSpace(stdout.String()),
		version,
		p.versionCacheKey(),
	); err != nil {
		logf(cmd.Stderr, "failed to write version to stdout: %v", err)
		return 1
	}

	return 0
}

func (cmd *mainCmd) toolExecRewrite(pkg string, p toolExecParams) (exitCode int) {
	// We only need to modify the arguments for "compile" calls which work with .go files.
	if !isCompile(p.Tool) {
		return cmd.runOriginal(p)
	}

	// We only modify files that import errtrace, so stdlib is never eliglble.
	if isStdLib(p.ToolArgs) {
		return cmd.runOriginal(p)
	}

	exitCode, err := cmd.rewriteCompile(pkg, p)
	if err != nil {
		cmd.log.Print(err)
		return 1
	}

	return exitCode
}

func (cmd *mainCmd) rewriteCompile(pkg string, p toolExecParams) (exitCode int, _ error) {
	var canRewrite, needRewrite bool
	parsed := make(map[string]parsedFile)
	for _, arg := range p.ToolArgs {
		if !isGoFile(arg) {
			continue
		}

		contents, err := os.ReadFile(arg)
		if err != nil {
			return -1, errtrace.Wrap(err)
		}

		f, err := cmd.parseFile(arg, contents, rewriteOpts{})
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

	if !needRewrite {
		return cmd.runOriginal(p), nil
	}

	if !canRewrite {
		if p.requiredPackage(pkg) {
			logf(cmd.Stderr, "errtrace required package %v missing errtrace import, needs rewrite", pkg)
			return 1, nil
		}
		return cmd.runOriginal(p), nil
	}

	// Use a temporary directory per-package that is rewritten.
	tempDir, err := os.MkdirTemp("", filepath.Base(pkg))
	if err != nil {
		return -1, errtrace.Wrap(err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // best-effort removal of temp files.

	newArgs := make([]string, 0, len(p.ToolArgs))
	for _, arg := range p.ToolArgs {
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

		// TODO: Handle clashes with the same base name in different directories (E.g., with bazel).
		newFile := filepath.Join(tempDir, filepath.Base(arg))
		if err := os.WriteFile(newFile, out.Bytes(), 0o666); err != nil {
			return -1, errtrace.Wrap(err)
		}

		newArgs = append(newArgs, newFile)
	}

	p.ToolArgs = newArgs
	return cmd.runOriginal(p), nil
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

func (cmd *mainCmd) runOriginal(p toolExecParams) (exitCode int) {
	tool := exec.Command(p.Tool, p.ToolArgs...)
	tool.Stdin = cmd.Stdin
	tool.Stdout = cmd.Stdout
	tool.Stderr = cmd.Stderr

	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		logf(cmd.Stderr, "tool %v failed: %v", p.Tool, err)
		return 1
	}

	return 0
}

// binaryVersion returns a string that uniquely identifies the binary.
// We prefer to use the VCS info embedded in the build if possible
// falling back to the MD5 of the binary.
func binaryVersion() (string, error) {
	sha, ok := readBuildSHA()
	if ok {
		return sha, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", errtrace.Wrap(err)
	}

	contents, err := os.ReadFile(exe)
	if err != nil {
		return "", errtrace.Wrap(err)
	}

	binaryHash := md5.Sum(contents)
	return hex.EncodeToString(binaryHash[:]), nil
}

// readBuildSHA returns the VCS SHA, if it's from an unmodified VCS state.
func readBuildSHA() (_ string, ok bool) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	var sha string
	for _, s := range buildInfo.Settings {
		switch s.Key {
		case "vcs.revision":
			sha = s.Value
		case "vcs.modified":
			if s.Value != "false" {
				return "", false
			}
		}
	}
	return sha, sha != ""
}

// isStdLib checks if the current execution is for stdlib.
func isStdLib(args []string) bool {
	return slices.Contains(args, "-std")
}

func packageSelectorMatch(selector, importPath string) bool {
	if pkgPrefix, ok := strings.CutSuffix(selector, "..."); ok {
		// foo/... should match foo, but not foobar so we want
		// the pkgPrefix to contain the /.
		if strings.TrimSuffix(pkgPrefix, "/") == importPath {
			return true
		}
		return strings.HasPrefix(importPath, pkgPrefix)
	}

	return selector == importPath
}
