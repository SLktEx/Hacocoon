package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var forbiddenStateMutations = map[string]struct{}{
	"PutEnvironment":        {},
	"AcquireWorkspaceLease": {},
	"PutWorkspaceLease":     {},
	"DeleteWorkspaceLease":  {},
}

var infrastructureCommands = map[string]struct{}{
	"incus":   {},
	"nft":     {},
	"ip":      {},
	"wsl.exe": {},
}

// TestApplicationCodeUsesCanonicalLifecycleAndProviderBoundaries keeps the
// repository itself on the safe path. A future feature or coding agent should
// not be able to bypass the Environment lifecycle transaction by composing
// low-level state mutations, or bypass provider/platform adapters by spawning
// privileged infrastructure tools from application code.
func TestApplicationCodeUsesCanonicalLifecycleAndProviderBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	fset := token.NewFileSet()
	var violations []string

	for _, subtree := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, subtree), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if strings.HasPrefix(filepath.ToSlash(rel), "internal/state/") {
				return nil
			}

			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
					if _, forbidden := forbiddenStateMutations[selector.Sel.Name]; forbidden {
						position := fset.Position(call.Pos())
						violations = append(violations, position.String()+": direct low-level Environment/Workspace state mutation "+selector.Sel.Name+" bypasses the lifecycle transition API")
					}
					if selector.Sel.Name == "Run" && len(call.Args) >= 2 {
						if command, ok := stringLiteral(call.Args[1]); ok {
							if _, forbidden := infrastructureCommands[command]; forbidden {
								position := fset.Position(call.Pos())
								violations = append(violations, position.String()+": direct infrastructure command "+strconv.Quote(command)+" belongs behind a provider/platform adapter")
							}
						}
					}
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "Command" && len(call.Args) > 0 {
					if command, ok := stringLiteral(call.Args[0]); ok {
						if _, forbidden := infrastructureCommands[command]; forbidden {
							position := fset.Position(call.Pos())
							violations = append(violations, position.String()+": direct infrastructure command "+strconv.Quote(command)+" belongs behind a provider/platform adapter")
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("inspect %s architecture: %v", subtree, err)
		}
	}

	if len(violations) != 0 {
		t.Fatalf("architecture boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}
