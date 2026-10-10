package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/SLktEx/Hacocoon/internal/state"
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
			// Native commands belong to these implementation owners. Lifecycle
			// mutation checks still apply inside the adapters and platform code.
			providerCommands := strings.HasPrefix(filepath.ToSlash(rel), "internal/adapters/incus/") ||
				strings.HasPrefix(filepath.ToSlash(rel), "internal/platform/")

			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			violations = append(violations, sourceBoundaryViolations(fset, file, providerCommands)...)
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

func sourceBoundaryViolations(fset *token.FileSet, file *ast.File, providerCommands bool) []string {
	var violations []string
	imports := execImportNames(file)
	shadows := execImportShadows(file, imports)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		position := fset.Position(call.Pos()).String()
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			if _, forbidden := forbiddenStateMutations[selector.Sel.Name]; forbidden {
				violations = append(violations, position+": direct low-level Environment/Workspace state mutation "+selector.Sel.Name+" bypasses the lifecycle transition API")
			}
		}
		if !providerCommands {
			if command, ok := infrastructureCommand(call, imports, shadows); ok {
				violations = append(violations, position+": direct infrastructure command "+strconv.Quote(command)+" belongs behind a provider/platform adapter")
			}
		}
		return true
	})
	return violations
}

func execImportNames(file *ast.File) map[string]bool {
	names := make(map[string]bool)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "os/exec" {
			continue
		}
		name := "exec"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "_" {
			names[name] = true
		}
	}
	return names
}

// Keep the existing runner.Run convention. The os/exec check is deliberately
// limited to direct imported calls with literal command names: dynamic names,
// function values and wrappers require other checks or review, not call tracing.
func infrastructureCommand(call *ast.CallExpr, imports map[string]bool, shadows commandShadows) (string, bool) {
	var argument ast.Expr
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Run" && len(call.Args) >= 2 {
		argument = call.Args[1]
	} else {
		name := execFunctionName(call.Fun, imports, shadows)
		switch {
		case name == "Command" && len(call.Args) > 0:
			argument = call.Args[0]
		case name == "CommandContext" && len(call.Args) > 1:
			argument = call.Args[1]
		}
	}
	command, literal := stringLiteral(argument)
	_, forbidden := infrastructureCommands[command]
	return command, literal && forbidden
}

func execFunctionName(fun ast.Expr, imports map[string]bool, shadows commandShadows) string {
	switch fun := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		receiver, ok := fun.X.(*ast.Ident)
		if ok && imports[receiver.Name] && !shadows.contains(receiver.Name, receiver.Pos()) {
			return fun.Sel.Name
		}
	case *ast.Ident:
		if imports["."] && !shadows.contains(fun.Name, fun.Pos()) {
			return fun.Name
		}
	}
	return ""
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

// The production catalog must not offer an alternate path that can mutate one
// side of Environment ownership, including to provider/plugin callers outside
// the application-source scan above.
func TestCatalogExposesOnlyAggregateEnvironmentMutations(t *testing.T) {
	catalog := reflect.TypeOf(state.NewEnvironmentJSONStore(""))
	for _, name := range []string{"PutEnvironment", "DeleteEnvironment", "AcquireWorkspaceLease", "PutWorkspaceLease", "DeleteWorkspaceLease"} {
		if _, exists := catalog.MethodByName(name); exists {
			t.Errorf("catalog exposes independent mutation %s", name)
		}
	}
}
