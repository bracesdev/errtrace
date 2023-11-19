// errtrace instruments Go code with error return tracing.
//
// # Usage
//
//	errtrace [options] <source files>
//
// This will transform the source files and write them to the standard output.
// Use the following options to control the output:
//
//   - -w: write result to the given source files instead of stdout
package main

// TODO
//   - -toolexec: run as a tool executor, fit for use with 'go build -toolexec'

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	cmd := &mainCmd{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
	}
	os.Exit(cmd.Run(os.Args[1:]))
}

type mainParams struct {
	Write bool     // -w
	Files []string // list of files to process
}

func (p *mainParams) Parse(w io.Writer, args []string) error {
	flag := flag.NewFlagSet("errtrace", flag.ContinueOnError)
	flag.SetOutput(w)
	flag.Usage = func() {
		fmt.Fprintln(w, "usage: errtrace [options] <source files>")
		flag.PrintDefaults()
	}

	flag.BoolVar(&p.Write, "w", false,
		"write result to the given source files instead of stdout")
	// TODO: toolexec mode

	if err := flag.Parse(args); err != nil {
		return err
	}

	p.Files = flag.Args()
	if len(p.Files) == 0 {
		flag.Usage()
		return errors.New("no source files")
	}

	return nil
}

type mainCmd struct {
	Stderr io.Writer
	Stdout io.Writer

	log *log.Logger
}

func (cmd *mainCmd) Run(args []string) (exitCode int) {
	cmd.log = log.New(cmd.Stderr, "", 0)

	var p mainParams
	if err := p.Parse(cmd.Stderr, args); err != nil {
		cmd.log.Println("errtrace:", err)
		return 1
	}

	for _, file := range p.Files {
		if err := cmd.processFile(p.Write, file); err != nil {
			cmd.log.Printf("%s:%s", file, err)
			exitCode = 1
		}
	}

	return exitCode
}

// processFile processes a single file.
// This operates in two phases:
//
// First, it walks the AST to find all the places that need to be modified,
// extracting other information as needed.
// The list of edits is at a higher level than plain text modifications:
// it tracks the kind of edit to make semantically,
// e.g. "wrap an expression" rather than "add these words at this position."
// This provides freedom to gather information
// before committing to specific strings and names.
//
// The collected information is used to pick a package name,
// whether we need an import, etc. and *then* the edits are applied.
func (cmd *mainCmd) processFile(write bool, filename string) error {
	fset := token.NewFileSet()
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return err
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

	var edits []edit
	w := walker{
		fset:     fset,
		errtrace: errtracePkg,
		logger:   cmd.log,
		edits:    &edits,
	}
	ast.Walk(&w, f)

	if !importsErrtrace && len(edits) > 0 {
		var lastImportDecl *ast.GenDecl
		for _, imp := range f.Decls {
			decl, ok := imp.(*ast.GenDecl)
			if !ok || decl.Tok != token.IMPORT {
				break
			}
			lastImportDecl = decl
		}

		var edit appendImportEdit
		if lastImportDecl != nil {
			// import "foo"
			// // becomes
			// import "foo"; import "brace.dev/errtrace"
			edit.Node = lastImportDecl
		} else {
			// package foo
			// // becomes
			// package foo; import "brace.dev/errtrace"
			edit.Node = f.Name
		}
		edits = append(edits, &edit)
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].Start() < edits[j].Start()
	})

	// Detect overlapping edits.
	// This indicates a bug in the walker.
	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1], edits[i]
		if prev.End() > cur.Start() {
			var msg strings.Builder
			fmt.Fprintf(&msg, "%s:found overlapping edit:\n", filename)
			fmt.Fprintf(&msg, "\t%s:%v\n", fset.Position(prev.End()), prev)
			fmt.Fprintf(&msg, "\t%s:%v\n", fset.Position(cur.Start()), cur)
			panic(msg.String())
		}
	}

	outw := cmd.Stdout
	if write {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		outw = f
	}
	out := bufio.NewWriter(outw)
	defer out.Flush()

	var lastOffset int
	file := fset.File(f.Pos())
	for _, edit := range edits {
		start, end := file.Offset(edit.Start()), file.Offset(edit.End())
		_, _ = out.Write(src[lastOffset:start])
		lastOffset = end

		switch edit := edit.(type) {
		case *appendImportEdit:
			// Add the original node as-is.
			_, _ = out.Write(src[start:end])
			if errtracePkg == "errtrace" {
				// Don't use named imports if we're using the default name.
				fmt.Fprintf(out, "; import %q", "braces.dev/errtrace")
			} else {
				fmt.Fprintf(out, "; import %s %q", errtracePkg, "braces.dev/errtrace")
			}

		case *wrapEdit:
			fmt.Fprintf(out, "%s.Wrap(", errtracePkg)
			_, _ = out.Write(src[start:end])
			_, _ = out.WriteString(")")

		case *assignWrapEdit:
			// Turns this:
			//	return
			// Into this:
			//	x, y = errtrace.Wrap(x), errtrace.Wrap(y); return
			for i, name := range edit.Names {
				if i > 0 {
					_, _ = out.WriteString(", ")
				}
				fmt.Fprintf(out, "%s", name)
			}
			_, _ = out.WriteString(" = ")
			for i, name := range edit.Names {
				if i > 0 {
					_, _ = out.WriteString(", ")
				}
				fmt.Fprintf(out, "%s.Wrap(%s)", errtracePkg, name)
			}
			_, _ = out.WriteString("; return")

		default:
			cmd.log.Panicf("unhandled edit type %T", edit)
		}
	}
	_, _ = out.Write(src[lastOffset:]) // flush remaining

	return nil
}

type walker struct {
	// Inputs

	fset     *token.FileSet // file set for positional information
	errtrace string         // name of the errtrace package
	logger   *log.Logger

	// Outputs

	// edits is the list of edits to make.
	edits *[]edit

	// State

	numReturns   int      // number of return values
	returnNames  []string // names of return values, if any
	returnErrors []int    // indices of error return values
}

var _ ast.Visitor = (*walker)(nil)

func (t *walker) logf(pos token.Pos, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	t.logger.Printf("%s:%s", t.fset.Position(pos), msg)
}

func (t *walker) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.FuncDecl:
		return t.funcType(n.Type)

	case *ast.FuncLit:
		return t.funcType(n.Type)

	case *ast.ReturnStmt:
		// Doesn't return errors. Continue recursing.
		if len(t.returnErrors) == 0 {
			return t
		}

		// Naked return.
		// Add assignments to the named return values.
		if n.Results == nil {
			// TODO: record error names instead of return names
			errorNames := make([]string, len(t.returnErrors))
			for i, idx := range t.returnErrors {
				errorNames[i] = t.returnNames[idx]
			}

			*t.edits = append(*t.edits, &assignWrapEdit{
				Names: errorNames,
				Stmt:  n,
			})

			return nil
		}

		// Return with values.
		// Wrap each nth return value in-place
		// unless the return is a "return foo()" call
		// beacuse we can't wrap that.
		if len(n.Results) != t.numReturns {
			t.logf(n.Pos(), "return statement has %d results, expected %d",
				len(n.Results), t.numReturns)
			return nil
		}
	wrapLoop:
		for _, idx := range t.returnErrors {
			expr := n.Results[idx]

			switch expr := expr.(type) {
			case *ast.CallExpr:
				// Ignore if it's already errtrace.Wrap(...).
				if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
					if isIdent(sel.X, t.errtrace) && isIdent(sel.Sel, "Wrap") {
						continue wrapLoop
					}
				}

			case *ast.Ident:
				// Optimization: ignore if it's "nil".
				if expr.Name == "nil" {
					continue wrapLoop
				}
			}

			*t.edits = append(*t.edits, &wrapEdit{
				Expr: expr,
			})
		}
	}

	// TODO: handle "errtrace" symbol name collision.
	return t
}

func (t *walker) funcType(ft *ast.FuncType) ast.Visitor {
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
		names  []string // names of return values, if any
		errors []int    // indices of error return values
		count  int      // total number of return values
	)
	for _, field := range ft.Results.List {
		isError := isIdent(field.Type, "error")

		// field.Names is nil for unnamed return values.
		// Either all returns are named or none are.
		if len(field.Names) > 0 {
			// TODO: handle "_" names
			for _, name := range field.Names {
				names = append(names, name.Name)
				if isError {
					errors = append(errors, count)
				}
				count++
			}
		} else {
			if isError {
				errors = append(errors, count)
			}
			count++
		}
	}

	// If there are no error return values,
	// recurse to look for function literals.
	if len(errors) == 0 {
		return t
	}

	// Shallow copy with new state.
	newT := *t
	newT.returnNames = names
	newT.returnErrors = errors
	newT.numReturns = count
	return &newT
}

// edit is a request to modify a range of source code.
type edit interface {
	Start() token.Pos
	End() token.Pos
	String() string
}

// appendImportEdit adds an import declaration to the file
// right after the given node.
type appendImportEdit struct {
	Node ast.Node // the node to insert the import after
}

func (e *appendImportEdit) Start() token.Pos {
	return e.Node.Pos()
}

func (e *appendImportEdit) End() token.Pos {
	return e.Node.End()
}

func (e *appendImportEdit) String() string {
	return fmt.Sprintf("append errtrace import after %T", e.Node)
}

// wrapEdit adds a errtrace.Wrap call around an expression.
//
//	foo() -> errtrace.Wrap(foo())
//
// This will be used in a majority of the cases including
// assignments to named return values in deferred functions
type wrapEdit struct {
	Expr ast.Expr
}

func (e *wrapEdit) Start() token.Pos {
	return e.Expr.Pos()
}

func (e *wrapEdit) End() token.Pos {
	return e.Expr.End()
}

func (e *wrapEdit) String() string {
	return fmt.Sprintf("wrap %T", e.Expr)
}

// assignWrapEdit wraps a variable in-place with an errtrace.Wrap call.
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
type assignWrapEdit struct {
	Names []string
	Stmt  *ast.ReturnStmt // Stmt.Results == nil
}

func (e *assignWrapEdit) Start() token.Pos {
	return e.Stmt.Pos()
}

func (e *assignWrapEdit) End() token.Pos {
	return e.Stmt.End()
}

func (e *assignWrapEdit) String() string {
	return fmt.Sprintf("assign errors before %v", e.Names)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
