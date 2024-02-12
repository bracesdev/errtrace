// errtrace instruments Go code with error return tracing.
//
// # Installation
//
// Install errtrace with:
//
//	go install braces.dev/errtrace/cmd/errtrace@latest
//
// # Usage
//
//	errtrace [options] <source files | patterns>
//
// This will transform source files and write them to the standard output.
//
// If instead of source files, Go package patterns are given,
// errtrace will transform all the files that match those patterns.
// For example, 'errtrace ./...' will transform all files in the current
// package and all subpackages.
//
// Use the following flags to control the output:
//
//	-format
//	      whether to format ouput; one of: [auto, always, never].
//	      auto is the default and will format if the output is being written to a file.
//	-w    write result to the given source files instead of stdout.
//	-l    list files that would be modified without making any changes.
package main

// TODO
//   - -toolexec: run as a tool executor, fit for use with 'go build -toolexec'

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	gofmt "go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"braces.dev/errtrace"
)

func main() {
	cmd := &mainCmd{
		Stdin:  os.Stdin,
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}

	var exitCode int
	// panic(fmt.Sprintf("args: %v\ngot: %v", os.Args, os.Environ()))
	if toolExecPkg, ok := isToolExec(os.Args, os.Getenv); ok {
		if toolExecPkg == "" {
			exitCode = cmd.toolExecVersion(os.Args)
		} else {
			exitCode = cmd.RunToolExec(toolExecPkg, os.Args)
		}
	} else {
		fmt.Println("got", toolExecPkg, os.Environ())
		exitCode = cmd.Run(os.Args[1:])
	}
	os.Exit(exitCode)
}

type mainParams struct {
	Write    bool     // -w
	List     bool     // -l
	Format   format   // -format
	Patterns []string // list of files to process

	ImplicitStdin bool // whether stdin was picked because there were no args
}

func (p *mainParams) shouldFormat() bool {
	switch p.Format {
	case formatAuto:
		return p.Write
	case formatAlways:
		return true
	case formatNever:
		return false
	default:
		panic(fmt.Sprintf("unknown format %q", p.Format))
	}
}

func (p *mainParams) Parse(w io.Writer, args []string) error {
	flag := flag.NewFlagSet("errtrace", flag.ContinueOnError)
	flag.SetOutput(w)
	flag.Usage = func() {
		fmt.Fprintln(w, "usage: errtrace [options] <source files | patterns>")
		flag.PrintDefaults()
	}

	flag.Var(&p.Format, "format", "whether to format ouput; one of: [auto, always, never].\n"+
		"auto is the default and will format if the output is being written to a file.")
	flag.BoolVar(&p.Write, "w", false,
		"write result to the given source files instead of stdout.")
	flag.BoolVar(&p.List, "l", false,
		"list files that would be modified without making any changes.")

	// TODO: toolexec mode

	if err := flag.Parse(args); err != nil {
		return errtrace.Wrap(err)
	}

	p.Patterns = flag.Args()
	if len(p.Patterns) == 0 {
		// Read file from stdin when there's no args, similar to gofmt.
		p.Patterns = []string{"-"}
		p.ImplicitStdin = true
	}

	return nil
}

// format specifies whether the output should be gofmt'd.
type format int

var _ flag.Getter = (*format)(nil)

const (
	// formatAuto formats the output
	// if it's being written to a file
	// but not if it's being written to stdout.
	//
	// This is the default.
	formatAuto format = iota

	// formatAlways always formats the output.
	formatAlways

	// formatNever never formats the output.
	formatNever
)

func (f *format) Get() interface{} {
	return *f
}

// IsBoolFlag tells the flag package that plain "-format" is a valid flag.
// When "-format" is used without a value,
// the flag package will call Set("true") on the flag.
func (f *format) IsBoolFlag() bool {
	return true
}

func (f *format) Set(s string) error {
	switch s {
	case "auto":
		*f = formatAuto
	case "always", "true": // "true" comes from "-format" without a value
		*f = formatAlways
	case "never":
		*f = formatNever
	default:
		return errtrace.Wrap(fmt.Errorf("invalid format %q is not one of [auto, always, never]", s))
	}
	return nil
}

func (f *format) String() string {
	switch *f {
	case formatAuto:
		return "auto"
	case formatAlways:
		return "always"
	case formatNever:
		return "never"
	default:
		return fmt.Sprintf("format(%d)", *f)
	}
}

type mainCmd struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	log *log.Logger
}

func (cmd *mainCmd) Run(args []string) (exitCode int) {
	cmd.log = log.New(cmd.Stderr, "", 0)

	var p mainParams
	if err := p.Parse(cmd.Stderr, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		cmd.log.Printf("errtrace: %+v", err)
		return 1
	}

	files, err := expandPatterns(p.Patterns)
	if err != nil {
		cmd.log.Printf("errtrace: %+v", err)
		return 1
	}

	// Paths will be printed relative to CWD.
	// Paths outside it will be printed as-is.
	var workDir string
	if wd, err := os.Getwd(); err == nil {
		workDir = wd + string(filepath.Separator)
	}

	for _, file := range files {
		display := file
		if workDir != "" {
			// Not using filepath.Rel
			// because we don't want any ".."s in the path.
			display = strings.TrimPrefix(file, workDir)
		}
		if display == "-" {
			display = "stdin"
		}

		req := fileRequest{
			Format:        p.shouldFormat(),
			Write:         p.Write,
			List:          p.List,
			Filename:      display,
			Filepath:      file,
			ImplicitStdin: p.ImplicitStdin,
		}
		if err := cmd.processFile(req); err != nil {
			cmd.log.Printf("%s:%+v", display, err)
			exitCode = 1
		}
	}

	return exitCode
}

// expandPatterns turns the given list of patterns and files
// into a list of paths to files.
//
// Arguments that are already files are returned as-is.
// Arguments that are patterns are expanded using 'go list'.
// As a special case for stdin, "-" is returned as-is.
func expandPatterns(args []string) ([]string, error) {
	var files, patterns []string
	for _, arg := range args {
		if arg == "-" {
			files = append(files, arg)
			continue
		}

		if info, err := os.Stat(arg); err == nil && !info.IsDir() {
			files = append(files, arg)
			continue
		}

		patterns = append(patterns, arg)
	}

	if len(patterns) > 0 {
		pkgFiles, err := goListFiles(patterns)
		if err != nil {
			return nil, errtrace.Wrap(fmt.Errorf("go list: %w", err))
		}

		files = append(files, pkgFiles...)
	}

	return files, nil
}

var _execCommand = exec.Command

func goListFiles(patterns []string) (files []string, err error) {
	// The -e flag makes 'go list' include erroneous packages.
	// This will even include packages that have all files excluded
	// by build constraints if explicitly requested.
	// (with "path/to/pkg" instead of "./...")
	args := []string{"list", "-find", "-e", "-json"}
	args = append(args, patterns...)

	var stderr bytes.Buffer
	cmd := _execCommand("go", args...)
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("create stdout pipe: %w", err))
	}

	if err := cmd.Start(); err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("start command: %w", err))
	}

	type packageInfo struct {
		Dir            string
		GoFiles        []string
		CgoFiles       []string
		TestGoFiles    []string
		XTestGoFiles   []string
		IgnoredGoFiles []string
	}

	decoder := json.NewDecoder(stdout)
	for decoder.More() {
		var pkg packageInfo
		if err := decoder.Decode(&pkg); err != nil {
			return nil, errtrace.Wrap(fmt.Errorf("output malformed: %w", err))
		}

		for _, pkgFiles := range [][]string{
			pkg.GoFiles,
			pkg.CgoFiles,
			pkg.TestGoFiles,
			pkg.XTestGoFiles,
			pkg.IgnoredGoFiles,
		} {
			for _, f := range pkgFiles {
				files = append(files, filepath.Join(pkg.Dir, f))
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("%w\n%s", err, stderr.String()))
	}

	return files, nil
}

type fileRequest struct {
	Format bool
	Write  bool
	List   bool

	ReadFile string // if set, used instead of filename
	Filename string // name displayed to the user
	Filepath string // actual location on disk, or "-" for stdin

	ImplicitStdin bool
}

// processFile processes a single file.
// This operates in two phases:
//
// First, it walks the AST to find all the places that need to be modified,
// extracting other information as needed.
//
// The collected information is used to pick a package name,
// whether we need an import, etc. and *then* the edits are applied.
func (cmd *mainCmd) processFile(r fileRequest) error {
	fset := token.NewFileSet()

	src, err := cmd.readFile(r)
	if err != nil {
		return errtrace.Wrap(err)
	}

	f, err := parser.ParseFile(fset, r.Filename, src, parser.ParseComments)
	if err != nil {
		return errtrace.Wrap(err)
	}

	errtracePkg := "errtrace" // name to use for errtrace package
	var importsErrtrace bool  // whether the file imports errtrace already
	for _, imp := range f.Imports {
		if imp.Path.Value == `"braces.dev/errtrace"` {
			importsErrtrace = true
			if imp.Name != nil {
				// If the file already imports errtrace,
				// we'll want to use the name it's imported under.
				errtracePkg = imp.Name.Name
			}
			break
		}
	}

	if !importsErrtrace {
		// If the file doesn't import errtrace already,
		// do a quick check to find an unused identifier name.
		idents := make(map[string]struct{})
		ast.Inspect(f, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				idents[ident.Name] = struct{}{}
			}
			return true
		})

		// Pick a name that isn't already used.
		// Prefer "errtrace" if it's available.
		for i := 1; ; i++ {
			candidate := errtracePkg
			if i > 1 {
				candidate += strconv.Itoa(i)
			}

			if _, ok := idents[candidate]; !ok {
				errtracePkg = candidate
				break
			}
		}
	}

	var inserts []insert
	w := walker{
		fset:        fset,
		optouts:     optoutLines(fset, f.Comments),
		errtracePkg: errtracePkg,
		logger:      cmd.log,
		inserts:     &inserts,
	}
	ast.Walk(&w, f)

	// Look for unused optouts and warn about them.
	if len(w.optouts) > 0 {
		unusedOptouts := make([]int, 0, len(w.optouts))
		for line, used := range w.optouts {
			if used == 0 {
				unusedOptouts = append(unusedOptouts, line)
			}
		}
		sort.Ints(unusedOptouts)

		for _, line := range unusedOptouts {
			cmd.log.Printf("%s:%d:unused errtrace:skip", r.Filename, line)
		}
	}

	if r.List {
		if len(inserts) > 0 {
			_, err = fmt.Fprintf(cmd.Stdout, "%s\n", r.Filename)
		}
		return errtrace.Wrap(err)
	}

	// If errtrace isn't imported, but at least one insert was made,
	// we'll need to import errtrace.
	// Add an import declaration to the file.
	if !importsErrtrace && len(inserts) > 0 {
		// We want to insert the import after the last existing import.
		// If the last import is part of a group, we'll make it part of the group.
		//
		//	import (
		//		"foo"
		//	)
		//	// becomes
		//	import (
		//		"foo"; "brace.dev/errtrace"
		//	)
		//
		// Otherwise, we'll add a new import statement group.
		//
		//	import "foo"
		//	// becomes
		//	import "foo"; import "brace.dev/errtrace"
		var (
			lastImportSpec *ast.ImportSpec
			lastImportDecl *ast.GenDecl
		)
		for _, imp := range f.Decls {
			decl, ok := imp.(*ast.GenDecl)
			if !ok || decl.Tok != token.IMPORT {
				break
			}
			lastImportDecl = decl
			if decl.Lparen.IsValid() && len(decl.Specs) > 0 {
				// There's an import group.
				lastImportSpec, _ = decl.Specs[len(decl.Specs)-1].(*ast.ImportSpec)
			}
		}

		var i insertImportErrtrace
		switch {
		case lastImportSpec != nil:
			// import ("foo")
			i.At = lastImportSpec.End()
		case lastImportDecl != nil:
			// import "foo"
			i.At = lastImportDecl.End()
			i.AddKeyword = true
		default:
			// package foo
			i.At = f.Name.End()
			i.AddKeyword = true
		}
		inserts = append(inserts, &i)
	}

	sort.Slice(inserts, func(i, j int) bool {
		return inserts[i].Pos() < inserts[j].Pos()
	})

	out := bytes.NewBuffer(nil)

	var lastOffset int
	filePos := fset.File(f.Pos()) // position information for this file
	for _, it := range inserts {
		offset := filePos.Offset(it.Pos())
		_, _ = out.Write(src[lastOffset:offset])
		lastOffset = offset

		switch it := it.(type) {
		case *insertImportErrtrace:
			_, _ = io.WriteString(out, "; ")
			if it.AddKeyword {
				_, _ = io.WriteString(out, "import ")
			}

			if errtracePkg == "errtrace" {
				// Don't use named imports if we're using the default name.
				fmt.Fprintf(out, "%q", "braces.dev/errtrace")
			} else {
				fmt.Fprintf(out, "%s %q", errtracePkg, "braces.dev/errtrace")
			}

		case *insertWrapOpen:
			fmt.Fprintf(out, "%s.Wrap", errtracePkg)
			if it.N > 1 {
				fmt.Fprintf(out, "%d", it.N)
			}
			_, _ = out.WriteString("(")

		case *insertWrapClose:
			_, _ = out.WriteString(")")

		case *insertWrapAssign:
			// Turns this:
			//	return
			// Into this:
			//	x, y = errtrace.Wrap(x), errtrace.Wrap(y); return
			for i, name := range it.Names {
				if i > 0 {
					_, _ = out.WriteString(", ")
				}
				fmt.Fprintf(out, "%s", name)
			}
			_, _ = out.WriteString(" = ")
			for i, name := range it.Names {
				if i > 0 {
					_, _ = out.WriteString(", ")
				}
				fmt.Fprintf(out, "%s.Wrap(%s)", errtracePkg, name)
			}
			_, _ = out.WriteString("; ")

		default:
			cmd.log.Panicf("unhandled insertion type %T", it)
		}
	}
	_, _ = out.Write(src[lastOffset:]) // flush remaining

	outSrc := out.Bytes()
	if r.Format {
		outSrc, err = gofmt.Source(outSrc)
		if err != nil {
			return errtrace.Wrap(fmt.Errorf("format: %w", err))
		}
	}

	if r.Write {
		err = os.WriteFile(r.Filename, outSrc, 0o644)
	} else {
		_, err = cmd.Stdout.Write(outSrc)
	}
	return errtrace.Wrap(err)
}

func (cmd *mainCmd) readFile(r fileRequest) ([]byte, error) {
	if r.ReadFile != "" {
		return errtrace.Wrap2(os.ReadFile(r.ReadFile))
	}
	if r.Filepath != "-" {
		return errtrace.Wrap2(os.ReadFile(r.Filename))
	}

	if r.Write {
		return nil, errtrace.Wrap(fmt.Errorf("can't use -w with stdin"))
	}

	if r.ImplicitStdin {
		// Running with no args reads from stdin, but this is not obvious
		// so print a usage hint to stderr, if we think stdin is a TTY.
		// Best-effort check for a TTY by looking for a character device.
		type statter interface {
			Stat() (os.FileInfo, error)
		}
		if st, ok := cmd.Stdin.(statter); ok {
			if fi, err := st.Stat(); err == nil &&
				fi.Mode()&os.ModeCharDevice == os.ModeCharDevice {
				cmd.log.Println("reading from stdin; use '-h' for help")
			}
		}
	}

	return errtrace.Wrap2(io.ReadAll(cmd.Stdin))
}

type walker struct {
	// Inputs

	fset        *token.FileSet // file set for positional information
	errtracePkg string         // name of the errtrace package
	logger      *log.Logger

	optouts map[int]int // map from line to number of uses

	// Outputs

	// inserts is the list of inserts to make.
	inserts *[]insert

	// State

	// Function information:

	numReturns   int                      // number of return values
	errorIdents  []*ast.Ident             // identifiers for error return values (only if unnamed returns)
	errorObjs    map[*ast.Object]struct{} // objects for error return values (only if named returns)
	errorIndices []int                    // indices of error return values

	// Block information:

	// Errors that are wrapped in this block.
	alreadyWrapped map[*ast.Object]struct{}
	// The logic to detect re-wraps is pretty simplistic
	// since it doesn't do any control flow analysis.
	// If this becomes a necessity, we can add it later.
}

var _ ast.Visitor = (*walker)(nil)

func (t *walker) logf(pos token.Pos, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	t.logger.Printf("%s:%s", t.fset.Position(pos), msg)
}

func (t *walker) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.FuncDecl:
		return t.funcType(n, n.Type)

	case *ast.BlockStmt:
		newT := *t
		newT.alreadyWrapped = make(map[*ast.Object]struct{})
		return &newT

	case *ast.AssignStmt:
		t.assignStmt(n)

	case *ast.DeferStmt:
		// This is a bit inefficient;
		// we'll recurse into the DeferStmt's function literal (if any) twice.
		t.deferStmt(n)

	case *ast.FuncLit:
		return t.funcType(n, n.Type)

	case *ast.ReturnStmt:
		return t.returnStmt(n)
	}

	return t
}

func (t *walker) funcType(parent ast.Node, ft *ast.FuncType) ast.Visitor {
	// Clear state in case we're recursing into a function literal
	// inside a function that returns an error.
	newT := *t
	newT.errorObjs = nil
	newT.errorIdents = nil
	newT.errorIndices = nil
	newT.numReturns = 0
	t = &newT

	// If the function does not return anything,
	// we still need to recurse into any function literals.
	// Just return this visitor to continue recursing.
	if ft.Results == nil {
		return t
	}

	// If the function has return values,
	// we need to consider the following cases:
	//
	//   - no error return value
	//   - unnamed error return
	//   - named error return
	var (
		objs    []*ast.Object // objects of error return values
		idents  []*ast.Ident  // identifiers of named error return values
		indices []int         // indices of error return values
		count   int           // total number of return values
		// Invariants:
		//  len(indices) <= count
		//  len(names) == 0 || len(names) == len(indices)
	)
	for _, field := range ft.Results.List {
		isError := isIdent(field.Type, "error")

		// field.Names is nil for unnamed return values.
		// Either all returns are named or none are.
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				if isError {
					objs = append(objs, name.Obj)
					idents = append(idents, name)
					indices = append(indices, count)
				}
				count++
			}
		} else {
			if isError {
				indices = append(indices, count)
			}
			count++
		}
	}

	// If there are no error return values,
	// recurse to look for function literals.
	if len(indices) == 0 {
		return t
	}

	// If there's a single error return,
	// and this function is a method named "Unwrap",
	// don't wrap it so it plays nice with errors.Unwrap.
	if len(indices) == 1 {
		if decl, ok := parent.(*ast.FuncDecl); ok {
			if decl.Recv != nil && isIdent(decl.Name, "Unwrap") {
				return t
			}
		}
	}

	newT.errorObjs = setOf(objs)
	newT.errorIdents = idents
	newT.errorIndices = indices
	newT.numReturns = count
	return &newT
}

func (t *walker) returnStmt(n *ast.ReturnStmt) ast.Visitor {
	// Doesn't return errors. Continue recursing.
	if len(t.errorIndices) == 0 {
		return t
	}

	// Naked return.
	// We want to add assignments to the named return values.
	if n.Results == nil {
		if t.optout(n.Pos()) {
			return nil
		}

		// Ignore errors that have already been wrapped.
		names := make([]string, 0, len(t.errorIndices))
		for _, ident := range t.errorIdents {
			if _, ok := t.alreadyWrapped[ident.Obj]; ok {
				continue
			}
			names = append(names, ident.Name)
		}

		if len(names) > 0 {
			*t.inserts = append(*t.inserts, &insertWrapAssign{
				Names:  names,
				Before: n.Pos(),
			})
		}

		return nil
	}

	// Return with multiple return values being automatically expanded
	// E.g.,
	//	func foo() (int, error) {
	//		return bar()
	//	}
	// This needs to become:
	//	func foo() (int, error) {
	//		return Wrap2(bar())
	//	}
	// This is only supported if numReturns <= 6 and only the last return value is an error.
	if len(n.Results) == 1 && t.numReturns > 1 {
		if _, ok := n.Results[0].(*ast.CallExpr); !ok {
			t.logf(n.Pos(), "skipping function with incorrect number of return values: got %d, want %d", len(n.Results), t.numReturns)
			return t
		}

		switch {
		case t.numReturns > 6:
			t.logf(n.Pos(), "skipping function with too many return values")
		case len(t.errorIndices) != 1:
			t.logf(n.Pos(), "skipping function with multiple error returns")
		case t.errorIndices[0] != t.numReturns-1:
			t.logf(n.Pos(), "skipping function with non-final error return")
		default:
			t.wrapExpr(t.numReturns, n.Results[0])
		}

		return t
	}

	for _, idx := range t.errorIndices {
		t.wrapExpr(1, n.Results[idx])
	}

	return t
}

func (t *walker) assignStmt(n *ast.AssignStmt) {
	// Record assignments to named error return values.
	// We'll use this to detect re-wraps.
	for i, lhs := range n.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue // not an identifier
		}

		_, ok = t.errorObjs[ident.Obj]
		if !ok {
			continue // not an error assignment
		}

		if i < len(n.Rhs) && t.isErrtraceWrap(n.Rhs[i]) {
			// Assigning to a named error return value.
			t.alreadyWrapped[ident.Obj] = struct{}{}
		}
	}
}

func (t *walker) deferStmt(n *ast.DeferStmt) {
	// If there's a defer statement with a function literal,
	// *and* this function has named return values,
	// we'll want to watch for assignments to those return values.

	if len(t.errorIdents) == 0 {
		return // no named returns
	}

	funcLit, ok := n.Call.Fun.(*ast.FuncLit)
	if !ok {
		return // not a function literal
	}

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue // not an identifier
			}

			if _, ok := t.errorObjs[ident.Obj]; !ok {
				continue // not an error assignment
			}

			// Assigning to a named error return value.
			// Wrap the rhs in-place.
			t.wrapExpr(1, assign.Rhs[i])
		}

		return true
	})
}

func (t *walker) wrapExpr(n int, expr ast.Expr) {
	switch {
	case t.isErrtraceWrap(expr):
		return // already wrapped

	case isIdent(expr, "nil"):
		// Optimization: ignore if it's "nil".
		return
	}

	if t.optout(expr.Pos()) {
		return
	}

	*t.inserts = append(*t.inserts,
		&insertWrapOpen{N: n, Before: expr.Pos()},
		&insertWrapClose{After: expr.End()},
	)
}

// Detects if an expression is in the form errtrace.Wrap(e) or errtrace.Wrap{n}(e).
func (t *walker) isErrtraceWrap(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Ignore if it's already errtrace.Wrap(...).
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if !isIdent(sel.X, t.errtracePkg) {
		return false
	}

	return strings.HasPrefix(sel.Sel.Name, "Wrap") ||
		sel.Sel.Name == "New" ||
		sel.Sel.Name == "Errorf"
}

// optout reports whether the line at the given position
// is opted out of tracing, incrementing uses if so.
func (t *walker) optout(pos token.Pos) bool {
	line := t.fset.Position(pos).Line
	_, ok := t.optouts[line]
	if ok {
		t.optouts[line]++
	}
	return ok
}

// insert is a request to add something to the source code.
type insert interface {
	Pos() token.Pos // position to insert at
	String() string // description for debugging
}

// insertImportErrtrace adds an import declaration to the file
// right after the given node.
type insertImportErrtrace struct {
	AddKeyword bool      // whether the "import" keyword should be added
	At         token.Pos // position to insert at
}

func (e *insertImportErrtrace) Pos() token.Pos {
	return e.At
}

func (e *insertImportErrtrace) String() string {
	if e.AddKeyword {
		return "add import statement"
	}
	return "add import"
}

// insertWrapOpen adds a errtrace.Wrap call before an expression.
//
//	foo() -> errtrace.Wrap(foo()
//
// This needs a corresponding insertWrapClose to close the call.
type insertWrapOpen struct {
	// N specifies the number of parameters the Wrap function takes.
	// Defaults to 1.
	N int

	Before token.Pos // position to insert before
}

func (e *insertWrapOpen) Pos() token.Pos {
	return e.Before
}

func (e *insertWrapOpen) String() string {
	return "<errtrace.Wrap>"
}

// insertWrapClose closes a errtrace.Wrap call.
//
//	foo() -> foo())
//
// This needs a corresponding insertWrapOpen to open the call.
type insertWrapClose struct {
	After token.Pos // position to insert after
}

func (e *insertWrapClose) Pos() token.Pos {
	return e.After
}

func (e *insertWrapClose) String() string {
	return "</errtrace.Wrap>"
}

// insertWrapAssign wraps a variable in-place with an errtrace.Wrap call.
// This is used for naked returns in functions with named return values
//
// For example, it will turn this:
//
//	func foo() (err error) {
//		// ...
//		return
//	}
//
// Into this:
//
//	func foo() (err error) {
//		// ...
//		err = errtrace.Wrap(err); return
//	}
type insertWrapAssign struct {
	Names  []string  // names of variables to wrap
	Before token.Pos // position to insert before
}

func (e *insertWrapAssign) Pos() token.Pos {
	return e.Before
}

func (e *insertWrapAssign) String() string {
	return fmt.Sprintf("assign errors before %v", e.Names)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func setOf[T comparable](xs []T) map[T]struct{} {
	if len(xs) == 0 {
		return nil
	}

	set := make(map[T]struct{})
	for _, x := range xs {
		set[x] = struct{}{}
	}
	return set
}

var _errtraceSkip = regexp.MustCompile(`(^| )//errtrace:skip($|[ \(])`)

// optoutLines returns the line numbers
// that have a comment in the form:
//
//	//errtrace:skip
//
// It may be followed by other text, e.g.,
//
//	//errtrace:skip // for reasons
func optoutLines(
	fset *token.FileSet,
	comments []*ast.CommentGroup,
) map[int]int {
	lines := make(map[int]int)
	for _, cg := range comments {
		if len(cg.List) > 1 {
			// skip multiline comments which are full line comments, not tied to a return.
			continue
		}

		c := cg.List[0]
		if _errtraceSkip.MatchString(c.Text) {
			lineNo := fset.Position(c.Pos()).Line
			lines[lineNo] = 0
		}
	}
	return lines
}
