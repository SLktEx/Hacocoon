package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Parse every platform's sources, including entries excluded by this test host's
// build tags. Helpers keep separate process identities but share the thin-entry
// rule with the product CLI.
func TestCommandsContainOnlyEntrypointDelegation(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "cmd")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		if file.Name.Name != "main" || len(file.Imports) != 1 {
			t.Errorf("%s: command must import one implementation owner", path)
		}
		functions := 0
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.IMPORT {
					t.Errorf("%s: state belongs in implementation owner", path)
				}
			case *ast.FuncDecl:
				functions++
				if d.Name.Name != "main" || d.Body == nil || len(d.Body.List) != 1 {
					t.Errorf("%s: command must delegate directly from main", path)
					continue
				}
				stmt, ok := d.Body.List[0].(*ast.ExprStmt)
				if !ok {
					t.Errorf("%s: command contains control flow", path)
					continue
				}
				call, ok := stmt.X.(*ast.CallExpr)
				if !ok {
					t.Errorf("%s: command must call its owner", path)
					continue
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Main" || len(call.Args) != 0 {
					t.Errorf("%s: command must call owner.Main()", path)
				}
			}
		}
		if functions != 1 {
			t.Errorf("%s: command contains %d functions", path, functions)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoreDoesNotImportImplementations(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "internal", "core")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(name, "github.com/SLktEx/Hacocoon/") && name != "github.com/SLktEx/Hacocoon/internal/core" {
				t.Errorf("%s: Core imports implementation %s", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
