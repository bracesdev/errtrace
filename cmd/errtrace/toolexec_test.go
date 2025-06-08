package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"braces.dev/errtrace"
	"braces.dev/errtrace/internal/diff"
)

func TestToolExec(t *testing.T) {
	const testProgDir = "./testdata/toolexec-test"
	const testProgPkg = "braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/"

	errTraceCmd := filepath.Join(t.TempDir(), "errtrace")
	if runtime.GOOS == "windows" {
		errTraceCmd += ".exe" // can't run binaries on Windows otherwise.
	}
	_, stderr, err := runGo(t, ".", "build", "-o", errTraceCmd, ".")
	if err != nil {
		t.Fatalf("compile errtrace failed: %v\nstderr: %s", err, stderr)
	}

	wantTraces := tracePaths(t, testProgDir, "@trace")

	tests := []struct {
		name       string
		goArgs     func(t testing.TB) []string
		wantTraces []string
		wantErr    string
	}{
		{
			name: "no toolexec",
			goArgs: func(t testing.TB) []string {
				return []string{"."}
			},
			wantTraces: nil,
		},
		{
			name: "toolexec with pkg",
			goArgs: func(t testing.TB) []string {
				return []string{"-toolexec", errTraceCmd, "."}
			},
			wantTraces: wantTraces,
		},
		{
			name: "toolexec with files",
			goArgs: func(t testing.TB) []string {
				files, err := goListFiles([]string{testProgDir})
				if err != nil {
					t.Fatalf("list go files in %v: %v", testProgDir, err)
				}

				nonTest := slices.DeleteFunc(files, func(file string) bool {
					return strings.HasSuffix(file, "_test.go")
				})

				args := []string{"-toolexec", errTraceCmd}
				args = append(args, nonTest...)
				return args
			},
			wantTraces: wantTraces,
		},
		{
			name: "toolexec with required-packages ...",
			goArgs: func(t testing.TB) []string {
				return []string{"-toolexec", errTraceCmd + " -required-packages " + testProgPkg + "...", "."}
			},
			wantErr: "p1 missing errtrace import",
		},
		{
			name: "toolexec with required-packages list",
			goArgs: func(t testing.TB) []string {
				requiredPackages := strings.Join([]string{
					testProgPkg + "p2",
					testProgPkg + "p3",
				}, ",")
				return []string{"-toolexec", errTraceCmd + " -required-packages " + requiredPackages, "."}
			},
			wantTraces: wantTraces,
		},
		{
			name: "toolexec with unsafe-packages ...",
			goArgs: func(t testing.TB) []string {
				return []string{"-toolexec", errTraceCmd + " -unsafe-packages=...", "."}
			},
			wantTraces: append(wantTraces, tracePaths(t, testProgDir, "@unsafe-trace")...),
		},
		{
			name: "toolexec with unsafe-packages test/...",
			goArgs: func(t testing.TB) []string {
				return []string{"-toolexec", errTraceCmd + " -unsafe-packages=braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/...", "."}
			},
			wantTraces: append(wantTraces, tracePaths(t, testProgDir, "@unsafe-trace")...),
		},
		{
			name: "toolexec with unsafe-packages test/p1",
			goArgs: func(t testing.TB) []string {
				return []string{"-toolexec", errTraceCmd + " -unsafe-packages=braces.dev/errtrace/cmd/errtrace/testdata/toolexec-test/p1", "."}
			},
			wantTraces: append(wantTraces, tracePaths(t, testProgDir, "@unsafe-trace")...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.goArgs(t)

			verifyCompile := func(t testing.TB, _, stderr string, err error) {
				if tt.wantErr != "" {
					if err == nil {
						t.Fatalf("run expected error %v, but got no error", tt.wantErr)
						return
					}
					if !strings.Contains(stderr, tt.wantErr) {
						t.Fatalf("run unexpected error %q to contain %q", stderr, tt.wantErr)
					}
					return
				}

				if err != nil {
					t.Fatalf("run failed: %v\n%s", err, stderr)
				}
			}

			verifyTraces := func(t testing.TB, stdout string) {
				gotLines := fileLines(stdout)
				sort.Strings(gotLines)
				sort.Strings(tt.wantTraces)

				if d := diff.Diff(tt.wantTraces, gotLines); d != "" {
					t.Errorf("diff in traces:\n%s", d)
					t.Errorf("go run output:\n%s", stdout)
				}
			}

			t.Run("go run", func(t *testing.T) {
				runArgs := append([]string{"run"}, args...)
				stdout, stderr, err := runGo(t, testProgDir, runArgs...)
				verifyCompile(t, stdout, stderr, err)
				verifyTraces(t, stdout)
			})

			t.Run("go build", func(t *testing.T) {
				outExe := filepath.Join(t.TempDir(), "main")
				if runtime.GOOS == "windows" {
					outExe += ".exe"
				}

				runArgs := append([]string{"build", "-o", outExe}, args...)
				stdout, stderr, err := runGo(t, testProgDir, runArgs...)
				verifyCompile(t, stdout, stderr, err)
				if err != nil {
					return
				}

				cmd := exec.Command(outExe)
				output, err := cmd.Output()
				if err != nil {
					t.Fatalf("run built binary: %v", err)
				}
				verifyTraces(t, string(output))
			})
		})
	}

	// When using -unsafe-packages, packages that don't import errtrace can be
	// rewritten to use Wrap, but at least one package in the binary still needs
	// to import errtrace, otherwise, the final link will fail.
	t.Run("unsafe no errtrace import", func(t *testing.T) {
		args := []string{"run", "-toolexec", errTraceCmd + " -unsafe-packages=...", "."}
		_, stderr, err := runGo(t, "./testdata/toolexec-unsafe-no-import", args...)
		if err == nil {
			t.Fatal("run should fail")
		}
		if want := "relocation target braces.dev/errtrace.Wrap not defined"; !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing expected error: %v, got:\n%s", want, stderr)
		}
	})
}

func tracePaths(t testing.TB, path string, traceMarker string) []string {
	var wantTraces []string
	err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return errtrace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}

		for _, line := range findTraceLines(t, path, traceMarker) {
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Fatalf("abspath: %v", err)
			}
			if runtime.GOOS == "windows" {
				// On Windows, absPath uses windows path separators, e.g., "c:\foo"
				// but the paths reported in traces contain '/'.
				absPath = filepath.ToSlash(absPath)
			}

			wantTraces = append(wantTraces, fmt.Sprintf("%v:%v", absPath, line))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tracePaths failed: %v", err)
	}

	sort.Strings(wantTraces)
	return wantTraces
}

func findTraceLines(t testing.TB, file string, traceMarker string) []int {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck

	var traces []int
	scanner := bufio.NewScanner(f)
	var lineNum int
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, "// "+traceMarker) {
			traces = append(traces, lineNum)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	return traces
}

var fileLineRegex = regexp.MustCompile(`^\s*(.*:[0-9]+)$`)

func fileLines(out string) []string {
	var fileLines []string
	for _, line := range strings.Split(out, "\n") {
		if fileLineRegex.MatchString(line) {
			fileLines = append(fileLines, strings.TrimSpace(line))
		}
	}
	return fileLines
}

func runGo(t testing.TB, dir string, args ...string) (stdout, stderr string, _ error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), errtrace.Wrap(err)
}
