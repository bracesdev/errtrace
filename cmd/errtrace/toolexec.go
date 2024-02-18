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
	"strings"

	"braces.dev/errtrace"
)

func (cmd *mainCmd) handleToolExec(args []string) (exitCode int, handled bool) {
	// In toolexec mode, we're passed the original command + arguments.
	if len(args) == 0 {
		return -1, false
	}

	var p toolExecParams
	if err := p.Parse(os.Stdout, args); err != nil {
		cmd.log.Print(err)
		return 1, true
	}

	for _, arg := range args {
		if arg == "-V=full" {
			// compile is run first with "-V=full" to get a version number
			// for caching build IDs.
			// No TOOLEXEC_IMPORTPATH is set in this case.
			return cmd.toolExecVersion(p), true
		}
	}

	if cmd.Getenv == nil {
		cmd.Getenv = os.Getenv
	}
	// When "-toolexec" is used, the go cmd sets the package being compiled in the env.
	if pkg := cmd.Getenv("TOOLEXEC_IMPORTPATH"); pkg != "" {
		return cmd.toolExecRewrite(pkg, p), true
	}

	return -1, false
}

type toolExecParams struct {
	UnsafePkgSelectors []string

	Tool     string
	ToolArgs []string
}

func (p *toolExecParams) Parse(w io.Writer, args []string) error {
	flag := flag.NewFlagSet("errtrace (toolexec)", flag.ContinueOnError)
	flag.Usage = func() {
		fmt.Fprintln(w, `usage with go build/run/test: -toolexec="errtrace [options]"`)
		flag.PrintDefaults()
	}
	var unsafePkgs string
	flag.StringVar(&unsafePkgs, "unsafe-packages", "", "comma-separated list of package selectors "+
		"to unsafely rewrite, regardless of whether they import errtrace.")

	// Flag parsing stops at the first non-flag argument (no "-").
	if err := flag.Parse(args); err != nil {
		return errtrace.Wrap(err)
	}

	p.UnsafePkgSelectors = strings.Split(unsafePkgs, ",")

	remArgs := flag.Args()
	if len(remArgs) == 0 {
		return errtrace.New("toolexec expected tool arguments")
	}

	p.Tool = remArgs[0]
	p.ToolArgs = remArgs[1:]
	return nil
}

// Options affect the generated code, so use a hash
// of any options for the toolexec version.
func (p *toolExecParams) versionCacheKey() string {
	withoutTool := *p
	withoutTool.Tool = ""
	withoutTool.ToolArgs = nil

	optStr := fmt.Sprintf("%v", withoutTool)
	optHash := md5.Sum([]byte(optStr))
	return hex.EncodeToString(optHash[:])
}

func (cmd *mainCmd) toolExecVersion(p toolExecParams) int {
	version, err := binaryVersion()
	if err != nil {
		fmt.Fprintf(cmd.Stderr, "errtrace version failed: %v", err)
	}

	tool := exec.Command(p.Tool, p.ToolArgs...)
	var stdout bytes.Buffer
	tool.Stdout = &stdout
	tool.Stderr = cmd.Stderr
	if err := tool.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}

		fmt.Fprintf(cmd.Stderr, "tool %v failed: %v", p.Tool, err)
		return 1
	}

	fmt.Fprintf(cmd.Stdout, "%s-errtrace-%s%s\n", strings.TrimSpace(stdout.String()), version, p.versionCacheKey())
	return 0
}

func (p *toolExecParams) unsafeIncludesStd() bool {
	return slicesContains(p.UnsafePkgSelectors, "std")
}

func (p *toolExecParams) unsafeRewrite(importPath string) bool {
	for _, selector := range p.UnsafePkgSelectors {
		if packageSelectorMatch(selector, importPath) {
			return true
		}
	}
	return false
}

func (cmd *mainCmd) toolExecRewrite(pkg string, p toolExecParams) (exitCode int) {
	// We only need to modify the arguments for "compile" calls which work with .go files.
	if !isCompile(p.Tool) {
		return cmd.runOriginal(p)
	}

	// We only modify files that import errtrace, so stdlib is only eligible in unsafe mode.
	if isStdLib(p.ToolArgs) && !p.unsafeIncludesStd() {
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
	parsed := make(map[string]parsedFile)
	var importsErrtrace, hasInserts bool
	for _, arg := range p.ToolArgs {
		if !isGoFile(arg) {
			continue
		}

		contents, err := os.ReadFile(arg)
		if err != nil {
			return -1, errtrace.Wrap(err)
		}

		f, err := cmd.parseFile(arg, contents, rewriteOpts{
			NoWrapN: true,
		})
		if err != nil {
			return -1, errtrace.Wrap(err)
		}
		parsed[arg] = f

		if f.importsErrtrace {
			importsErrtrace = true
		}
		if len(f.inserts) > 0 {
			hasInserts = true
		}
	}

	unsafeForceImport := p.unsafeRewrite(pkg)
	if pkg == "braces.dev/errtrace" {
		unsafeForceImport = false
	}

	rewrite := hasInserts
	if !unsafeForceImport {
		rewrite = rewrite && importsErrtrace
	}

	if !rewrite {
		return cmd.runOriginal(p), nil
	}

	// Use a temporary directory per-package that is rewritten.
	tempDir, err := os.MkdirTemp("", filepath.Base(pkg))
	if err != nil {
		return -1, errtrace.Wrap(err)
	}
	// defer os.RemoveAll(tempDir) //nolint:errcheck // best-effort removal of temp files.

	// If a package doesn't already import errtrace, we'll use
	// go:linkname, which uses `errtrace_Wrap` style function names.

	addLinkName := !importsErrtrace

	newArgs := make([]string, 0, len(p.ToolArgs))
	for _, arg := range p.ToolArgs {
		f, ok := parsed[arg]
		if !ok || len(f.inserts) == 0 {
			newArgs = append(newArgs, arg)
			continue
		}

		if !importsErrtrace {
			f.unsafeImport = true
			f.errtracePkg = "errtrace"
			f.pkgSelector = "_"
		}

		// Add a //line directive so the original filepath is used in errors and panics.
		out := &bytes.Buffer{}
		_, _ = fmt.Fprintf(out, "//line %v:1\n", arg)

		if err := cmd.rewriteFile(f, out); err != nil {
			return -1, errtrace.Wrap(err)
		}

		if addLinkName {
			// TODO: Ensure symbol used for go:linkname isn't used.
			_, _ = fmt.Fprintln(out, "\n\n//go:linkname errtrace_Wrap braces.dev/errtrace.Wrap")
			_, _ = fmt.Fprintln(out, "func errtrace_Wrap(err error) error")
			addLinkName = false
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
		fmt.Fprintf(cmd.Stderr, "tool %v failed: %v", p.Tool, err)
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
	return slicesContains(args, "-std")
}

func packageSelectorMatch(selector, importPath string) bool {
	if pkgPrefix, ok := strings.CutSuffix(selector, "..."); ok {
		// foo/... also matches foo.
		pkgPrefix = strings.TrimSuffix(pkgPrefix, "/")
		return strings.HasPrefix(importPath, pkgPrefix)
	}

	return selector == importPath
}
