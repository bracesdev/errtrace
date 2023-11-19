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
	"strconv"
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
		fset:     fset,
		errtrace: errtracePkg,
		logger:   cmd.log,
		inserts:  &inserts,
	}
	ast.Walk(&w, f)

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
	filePos := fset.File(f.Pos()) // position information for this file
	for _, it := range inserts {
		offset := filePos.Offset(it.Pos())
		_, _ = out.Write(src[lastOffset:offset])
		lastOffset = offset

		switch it := it.(type) {
		case *insertImportErrtrace:
			// Add the original node as-is.
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
			fmt.Fprintf(out, "%s.Wrap(", errtracePkg)

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

	return nil
}

type walker struct {
	// Inputs

	fset     *token.FileSet // file set for positional information
	errtrace string         // name of the errtrace package
	logger   *log.Logger

	// Outputs

	// inserts is the list of inserts to make.
	inserts *[]insert

	// State

	numReturns   int                      // number of return values
	errorObjs    map[*ast.Object]struct{} // objects for error return values (only if named returns)
	errorNames   []string                 // names of error return values (only if named returns)
	errorIndices []int                    // indices of error return values
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

	case *ast.DeferStmt:
		// This is a bit inefficient;
		// we'll recurse into the DeferStmt's function literal (if any) twice.
		t.deferStmt(n)
		return t

	case *ast.FuncLit:
		return t.funcType(n, n.Type)

	case *ast.ReturnStmt:
		return t.returnStmt(n)
	}

	return t
}

func (t *walker) funcType(parent ast.Node, ft *ast.FuncType) ast.Visitor {
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
		names   []string      // names of error return values
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
					names = append(names, name.Name)
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

	// Shallow copy with new state.
	newT := *t
	newT.errorObjs = setOf(objs)
	newT.errorNames = names
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
	// Add assignments to the named return values.
	if n.Results == nil {
		*t.inserts = append(*t.inserts, &insertWrapAssign{
			Names:  t.errorNames,
			Before: n.Pos(),
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
	for _, idx := range t.errorIndices {
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

		*t.inserts = append(*t.inserts,
			&insertWrapOpen{Before: expr.Pos()},
			&insertWrapClose{After: expr.End()},
		)
	}

	return t
}

func (t *walker) deferStmt(n *ast.DeferStmt) {
	// If there's a defer statement with a function literal,
	// *and* this function has named return values,
	// we'll want to watch for assignments to those return values.

	if len(t.errorNames) == 0 || len(t.errorIndices) == 0 {
		return // no named returns or errors
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
			*t.inserts = append(*t.inserts,
				&insertWrapOpen{Before: assign.Rhs[i].Pos()},
				&insertWrapClose{After: assign.Rhs[i].End()},
			)
		}

		return true
	})
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
	} else {
		return "add import"
	}
}

// insertWrapOpen adds a errtrace.Wrap call before an expression.
//
//	foo() -> errtrace.Wrap(foo()
//
// This needs a corresponding insertWrapClose to close the call.
type insertWrapOpen struct {
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
