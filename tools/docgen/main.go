// Command docgen harvests the Go doc comments of the operator types and their FreepsFunctions
// and writes them as a generated Go file for the flowbuilder metadata API to read.
//
// It is the only code generation in this repository and it is deliberately optional: the
// generated file is committed, so a plain "go build" works without ever running this tool.
// Run "make generate" after adding or changing an operator or one of its doc comments.
//
// Usage:
//
//	go run ./tools/docgen -o connectors/flowbuilder/operatorDescriptions_generated.go
//
// The list of operators is taken from the availableOperators slice in freepsd/freepsd.go, so
// it matches what is actually registered. Operators registered elsewhere (the flow engine's
// own dynamic operators, the legacy ui and exec operators) are not covered and stay
// undocumented, which the API reports as an empty description.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	mainFile    = "freepsd/freepsd.go"
	operatorVar = "availableOperators"
)

func main() {
	repo := flag.String("repo", ".", "the repository root")
	out := flag.String("o", "", "the file to write, - for stdout")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "docgen: -o is required")
		os.Exit(1)
	}

	module, err := modulePath(*repo)
	if err != nil {
		fail(err)
	}

	ops, err := registeredOperators(*repo, module)
	if err != nil {
		fail(err)
	}

	harvested := make(map[string]map[string]string, len(ops))
	for _, op := range ops {
		pkg, err := newPackageCache(*repo, module).get(op.importDir)
		if err != nil {
			fail(fmt.Errorf("operator %v: %w", op.name, err))
		}
		harvested[op.name] = pkg.describe(op.typeName)
	}

	content := render(harvested)
	if *out == "-" {
		fmt.Print(content)
		return
	}
	if err := os.WriteFile(*out, []byte(content), 0o644); err != nil {
		fail(err)
	}
	funcs := 0
	for _, fns := range harvested {
		funcs += len(fns)
	}
	fmt.Printf("docgen: wrote %v with %v operator and %v function descriptions\n", *out, len(harvested), funcs)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "docgen:", err)
	os.Exit(1)
}

// operatorRef is one entry of the availableOperators slice.
type operatorRef struct {
	name      string // the freeps name, as the flow engine registers it
	typeName  string // the Go type name
	importDir string // the directory of the package that declares the type, relative to the repo
}

// registeredOperators reads the availableOperators slice from the main package and resolves
// every "&pkgAlias.TypeName{...}" entry to its type name and package directory.
func registeredOperators(repo, module string) ([]operatorRef, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(repo, mainFile), nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	imports := map[string]string{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		alias := filepath.Base(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports[alias] = path
	}

	var refs []operatorRef
	ast.Inspect(f, func(n ast.Node) bool {
		var lit *ast.CompositeLit
		switch stmt := n.(type) {
		case *ast.ValueSpec:
			if len(stmt.Names) != 1 || stmt.Names[0].Name != operatorVar || len(stmt.Values) != 1 {
				return true
			}
			lit, _ = stmt.Values[0].(*ast.CompositeLit)
		case *ast.AssignStmt:
			if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
				return true
			}
			ident, ok := stmt.Lhs[0].(*ast.Ident)
			if !ok || ident.Name != operatorVar {
				return true
			}
			lit, _ = stmt.Rhs[0].(*ast.CompositeLit)
		default:
			return true
		}
		if lit == nil {
			return true
		}
		for _, elt := range lit.Elts {
			unary, ok := elt.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				continue
			}
			inner, ok := unary.X.(*ast.CompositeLit)
			if !ok {
				continue
			}
			sel, ok := inner.Type.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				continue
			}
			importPath, found := imports[pkgIdent.Name]
			if !found {
				continue
			}
			typeName := sel.Sel.Name
			refs = append(refs, operatorRef{
				name:      freepsName(typeName),
				typeName:  typeName,
				importDir: strings.TrimPrefix(importPath, module+"/"),
			})
		}
		return false
	})

	if len(refs) == 0 {
		return nil, fmt.Errorf("no entries found for %v in %v", operatorVar, mainFile)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].name < refs[j].name })
	return refs, nil
}

// freepsName mirrors FreepsOperatorWrapper.GetName, which strips a leading "Operator" or "Op"
// from the type name. The flow engine lowercases the result.
func freepsName(typeName string) string {
	switch {
	case strings.HasPrefix(typeName, "Operator"):
		typeName = typeName[len("Operator"):]
	case strings.HasPrefix(typeName, "Op"):
		typeName = typeName[len("Op"):]
	}
	return strings.ToLower(typeName)
}

// goPackage is one parsed package directory.
type goPackage struct {
	fset  *token.FileSet
	files []*ast.File
}

type packageCache struct {
	repo   string
	module string
	parsed map[string]*goPackage
}

func newPackageCache(repo, module string) *packageCache {
	return &packageCache{repo: repo, module: module, parsed: map[string]*goPackage{}}
}

// get parses a package directory, using only the files that satisfy the build constraints of
// the default context. This matters because every connector with a build tag has a _dummy.go
// file that redeclares the same type without any doc comments.
func (c *packageCache) get(dir string) (*goPackage, error) {
	if pkg, ok := c.parsed[dir]; ok {
		return pkg, nil
	}
	bp, err := build.ImportDir(filepath.Join(c.repo, dir), 0)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range append(append([]string{}, bp.GoFiles...), bp.CgoFiles...) {
		f, err := parser.ParseFile(fset, filepath.Join(c.repo, dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	pkg := &goPackage{fset: fset, files: files}
	c.parsed[dir] = pkg
	return pkg, nil
}

// describe returns the doc comment of the given type and of every FreepsFunction method on it.
// The type name is used as the key for the operator description, the method names as keys for
// the function descriptions.
func (p *goPackage) describe(typeName string) map[string]string {
	descriptions := map[string]string{}
	for _, f := range p.files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if ok && ts.Name.Name == typeName {
						descriptions = describeType(descriptions, typeName, cleanComment(typeDoc(d, ts), typeName))
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil || len(d.Recv.List) != 1 || receiverName(d.Recv.List[0].Type) != typeName {
					continue
				}
				if !isFreepsFunction(p.fset, d) {
					continue
				}
				descriptions = describeType(descriptions, typeName, "")
				if doc := cleanComment(d.Doc, d.Name.Name); doc != "" {
					descriptions[d.Name.Name] = doc
				}
			}
		}
	}
	return descriptions
}

// describeType makes sure the operator itself has an entry, even when it has no doc comment,
// so the caller can tell "generated but empty" apart from "not generated at all".
func describeType(descriptions map[string]string, typeName, doc string) map[string]string {
	if _, ok := descriptions[""]; !ok {
		descriptions[""] = cleanComment(nil, typeName)
	}
	if doc != "" {
		descriptions[""] = doc
	}
	return descriptions
}

// typeDoc returns the doc comment of a type, which is either attached to the GenDecl or, for
// grouped type declarations, to the TypeSpec itself.
func typeDoc(d *ast.GenDecl, ts *ast.TypeSpec) *ast.CommentGroup {
	if d.Doc != nil && len(d.Specs) == 1 {
		return d.Doc
	}
	return ts.Doc
}

// receiverName returns the type name of a method receiver, unwrapping the pointer.
func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// isFreepsFunction mirrors base.getFreepsFunctionType: the method must return exactly one
// *base.OperatorIO and must not be one of the reserved *Suggestions helpers.
func isFreepsFunction(fset *token.FileSet, d *ast.FuncDecl) bool {
	if strings.HasSuffix(d.Name.Name, "Suggestions") {
		return false
	}
	if d.Type.Results == nil || len(d.Type.Results.List) != 1 {
		return false
	}
	returnType := &strings.Builder{}
	_ = printer.Fprint(returnType, fset, d.Type.Results.List[0].Type)
	return returnType.String() == "*base.OperatorIO"
}

// cleanComment returns the comment as a single line, with the leading identifier that Go's
// doc convention puts there removed, together with a simple copula: "OpAlert is a FreepsOperator
// that ..." becomes "FreepsOperator that ...", "SilenceAlert keeps the alert ..." becomes
// "keeps the alert ...".
func cleanComment(doc *ast.CommentGroup, subject string) string {
	if doc == nil {
		return ""
	}
	text := strings.Join(strings.Split(strings.TrimSpace(doc.Text()), "\n"), " ")
	text = strings.TrimSpace(strings.TrimPrefix(text, subject))
	for _, copula := range []string{"is a ", "is an ", "is the ", "is that ", "is "} {
		if rest, ok := strings.CutPrefix(text, copula); ok {
			text = rest
			break
		}
	}
	return strings.TrimSpace(text)
}

// render writes the descriptions as a Go source file. The empty string key of an operator is
// its own description, the other keys are its function names.
func render(harvested map[string]map[string]string) string {
	var b strings.Builder
	b.WriteString("// Code generated by tools/docgen. DO NOT EDIT.\n")
	b.WriteString("// Run \"make generate\" after changing an operator or its doc comments.\n\n")
	b.WriteString("package flowbuilder\n\n")

	names := make([]string, 0, len(harvested))
	for name := range harvested {
		names = append(names, name)
	}
	sort.Strings(names)

	b.WriteString("// operatorDescriptions maps the lowercase operator name to its description.\n")
	b.WriteString("var operatorDescriptions = map[string]string{\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\t%v: %v,\n", strconv.Quote(name), strconv.Quote(harvested[name][""]))
	}
	b.WriteString("}\n\n")

	b.WriteString("// functionDescriptions maps the lowercase operator name to the descriptions of\n")
	b.WriteString("// its functions, keyed by the function name as GetFunctions returns it.\n")
	b.WriteString("var functionDescriptions = map[string]map[string]string{\n")
	for _, name := range names {
		fns := harvested[name]
		keys := make([]string, 0, len(fns))
		for k := range fns {
			if k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		fmt.Fprintf(&b, "\t%v: {\n", strconv.Quote(name))
		for _, k := range keys {
			fmt.Fprintf(&b, "\t\t%v: %v,\n", strconv.Quote(k), strconv.Quote(fns[k]))
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")

	return b.String()
}

// modulePath reads the module path from go.mod.
func modulePath(repo string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in go.mod")
}
