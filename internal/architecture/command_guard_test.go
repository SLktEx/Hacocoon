package architecture_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// These fixtures are only parsed. They must never be compiled or executed.
func TestInfrastructureCommandImports(t *testing.T) {
	forms := []struct {
		name    string
		imports string
		call    string
	}{
		{"command", `import "os/exec"`, `exec.Command(COMMAND)`},
		{"context", `import "os/exec"`, `exec.CommandContext(ctx, COMMAND)`},
		{"alias command", `import process "os/exec"`, `process.Command(COMMAND)`},
		{"alias context", `import process "os/exec"`, `process.CommandContext(ctx, COMMAND)`},
		{"dot command", `import . "os/exec"`, `Command(COMMAND)`},
		{"dot context", `import . "os/exec"`, `CommandContext(ctx, COMMAND)`},
		{"parenthesized command", `import "os/exec"`, `(exec.Command)(COMMAND)`},
		{"parenthesized dot context", `import . "os/exec"`, `(CommandContext)(ctx, COMMAND)`},
	}
	for _, command := range []string{"incus", "nft", "ip", "wsl.exe"} {
		for _, form := range forms {
			t.Run(command+"/"+form.name, func(t *testing.T) {
				source := "package fixture\n" + form.imports + `
import "context"
func neverCalled(ctx context.Context) { _ = ` + strings.ReplaceAll(form.call, "COMMAND", `"`+command+`"`) + " }"
				assertSourceBoundaryViolations(t, source, false, []string{`direct infrastructure command "` + command + `"`})
			})
		}
	}
}

func TestInfrastructureCommandScope(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "allowed command and arguments",
			source: `package fixture
import "os/exec"
func neverCalled() { _ = exec.Command("echo", "incus") }`,
		},
		{
			name: "context command position",
			source: `package fixture
import ("context"; "os/exec")
func neverCalled(ctx context.Context) { _ = exec.CommandContext(ctx, "echo", "nft") }`,
		},
		{
			name:   "raw string literal",
			source: "package fixture\nimport \"os/exec\"\nfunc neverCalled() { _ = exec.Command(`incus`) }",
			want:   []string{`direct infrastructure command "incus"`},
		},
		{
			name: "escaped string literal",
			source: `package fixture
import "os/exec"
func neverCalled() { _ = exec.Command("\x69ncus") }`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "unrelated import",
			source: `package fixture
import exec "example.org/other"
func neverCalled() { _ = exec.Command("incus") }`,
		},
		{
			name: "unrelated function",
			source: `package fixture
func Command(label string) string { return label }
func neverCalled() { _ = Command("incus") }`,
		},
		{
			name: "unrelated context function",
			source: `package fixture
func CommandContext(context, label string) string { return label }
func neverCalled() { _ = CommandContext("context", "nft") }`,
		},
		{
			name: "unrelated method",
			source: `package fixture
type local struct{}
func (local) Command(label string) string { return label }
func neverCalled(exec local) { _ = exec.Command("incus") }`,
		},
		{
			name: "blank import is not a binding",
			source: `package fixture
import _ "os/exec"
func Command(label string) string { return label }
func neverCalled() { _ = Command("incus") }`,
		},
		{
			name: "parameter shadows import",
			source: `package fixture
import "os/exec"
var _ = exec.Command
type local struct { Command func(string) string }
func neverCalled(exec local) { _ = exec.Command("incus") }`,
		},
		{
			name: "short declaration shadows alias",
			source: `package fixture
import process "os/exec"
var _ = process.Command
type local struct { Command func(string) string }
func neverCalled() { process := local{}; _ = process.Command("incus") }`,
		},
		{
			name: "named result shadows alias",
			source: `package fixture
import process "os/exec"
var _ = process.Command
type local struct { Command func(string) string }
func neverCalled() (process local) { _ = process.Command("incus"); return }`,
		},
		{
			name: "nested shadow ends with block",
			source: `package fixture
import "os/exec"
type local struct { Command func(string) string }
func neverCalled() {
    { exec := local{}; _ = exec.Command("nft") }
    _ = exec.Command("incus")
}`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "declaration does not shadow its initializer",
			source: `package fixture
import "os/exec"
func neverCalled() { exec := exec.Command("incus"); _ = exec }`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "dot import shadowed by parameter",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
func neverCalled(Command func(string) string) { _ = Command("incus") }`,
		},
		{
			name: "dot context shadowed by local function",
			source: `package fixture
import . "os/exec"
var _ = Command
func neverCalled() {
    CommandContext := func(context, label string) string { return label }
    _ = CommandContext("context", "nft")
}`,
		},
		{
			name: "missing command arguments do not panic",
			source: `package fixture
import "os/exec"
func neverCalled() { exec.Command(); exec.CommandContext(nil) }`,
		},
		{
			name: "dynamic names remain outside literal scope",
			source: `package fixture
import "os/exec"
func neverCalled(command string) { _ = exec.Command(command) }`,
		},
		{
			name: "constant names remain outside literal scope",
			source: `package fixture
import "os/exec"
const command = "incus"
func neverCalled() { _ = exec.Command(command) }`,
		},
		{
			name: "indirect function values remain outside direct call scope",
			source: `package fixture
import "os/exec"
func neverCalled() { command := exec.Command; _ = command("incus") }`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSourceBoundaryViolations(t, test.source, false, test.want)
		})
	}
}

func TestSourceBoundaryControls(t *testing.T) {
	source := `package fixture
import ("context"; "os/exec")
type runner interface { Run(context.Context, string) error }
type state interface {
    PutEnvironment()
    AcquireWorkspaceLease()
    PutWorkspaceLease()
    DeleteWorkspaceLease()
}
func neverCalled(ctx context.Context, r runner, s state) {
    _ = exec.Command("incus")
    _ = exec.CommandContext(ctx, "nft")
    _ = r.Run(ctx, "ip")
    s.PutEnvironment()
    s.AcquireWorkspaceLease()
    s.PutWorkspaceLease()
    s.DeleteWorkspaceLease()
}`
	mutations := []string{
		"direct low-level Environment/Workspace state mutation PutEnvironment",
		"direct low-level Environment/Workspace state mutation AcquireWorkspaceLease",
		"direct low-level Environment/Workspace state mutation PutWorkspaceLease",
		"direct low-level Environment/Workspace state mutation DeleteWorkspaceLease",
	}
	t.Run("application", func(t *testing.T) {
		want := append([]string{
			`direct infrastructure command "incus"`,
			`direct infrastructure command "nft"`,
			`direct infrastructure command "ip"`,
		}, mutations...)
		assertSourceBoundaryViolations(t, source, false, want)
	})
	t.Run("adapter or platform", func(t *testing.T) {
		assertSourceBoundaryViolations(t, source, true, mutations)
	})
}

func TestInfrastructureCommandShadowLifetimes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "variable declaration initializer and later shadow",
			body: `var exec = local{Command: func(string) string { _ = exec.Command("incus"); return "" }}
_ = exec.Command("nft")`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "short declaration reuse remains shadowed",
			body: `exec := local{}
exec, other := exec, 1
_ = other
_ = exec.Command("incus")`,
		},
		{
			name: "grouped variables bind after each specification",
			body: `var (
    exec = local{Command: func(string) string { _ = exec.Command("incus"); return "" }}
    value = exec.Command("nft")
)
_ = value`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "if initializer shadow includes else and ends after if",
			body: `if exec := (func() local { _ = exec.Command("incus"); return local{} })(); true {
    _ = exec.Command("nft")
} else { _ = exec.Command("ip") }
_ = exec.Command("wsl.exe")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "wsl.exe"`},
		},
		{
			name: "for initializer shadow includes condition and post",
			body: `for exec := (func() local { _ = exec.Command("incus"); return local{} })(); exec.Command("nft") != ""; exec.Command("ip") {
    _ = exec.Command("wsl.exe")
}
_ = exec.Command("incus")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "incus"`},
		},
		{
			name: "range expression before body binding",
			body: `for _, exec := range []local{{Command: func(string) string { _ = exec.Command("incus"); return "" }}} {
    _ = exec.Command("nft")
}
_ = exec.Command("ip")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "ip"`},
		},
		{
			name: "switch initializer shadow covers tag and clauses",
			body: `switch exec := (func() local { _ = exec.Command("incus"); return local{} })(); exec.Command("nft") {
case "": _ = exec.Command("ip")
default: _ = exec.Command("wsl.exe")
}
_ = exec.Command("incus")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "incus"`},
		},
		{
			name: "switch case locals do not shadow other cases",
			body: `switch {
case true: exec := local{}; _ = exec.Command("nft")
default: _ = exec.Command("incus")
}`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "select receive expression before clause binding",
			body: `select {
case exec := <- (func() chan local { _ = exec.Command("incus"); return nil })():
    _ = exec.Command("nft")
default: _ = exec.Command("ip")
}
_ = exec.Command("wsl.exe")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "ip"`, `direct infrastructure command "wsl.exe"`},
		},
		{
			name: "type switch guard before per-case binding",
			body: `switch exec := (func() any { _ = exec.Command("incus"); return local{} })().(type) {
case local: _ = exec.Command("nft")
case *exec.Cmd: _ = exec
default: _ = exec
}
_ = exec.Command("ip")`,
			want: []string{`direct infrastructure command "incus"`, `direct infrastructure command "ip"`},
		},
		{
			name: "closure parameter shadows only closure body",
			body: `(func(exec local) { _ = exec.Command("nft") })(local{})
_ = exec.Command("incus")`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "struct field does not shadow import",
			body: `value := struct { exec string }{exec: "label"}
_ = value
_ = exec.Command("incus")`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "label and function type parameters do not shadow import",
			body: `var callback func(exec local)
_ = callback
exec: for { _ = exec.Command("incus"); break exec }`,
			want: []string{`direct infrastructure command "incus"`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := `package fixture
import "os/exec"
var _ = exec.Command
type local struct { Command func(string) string }
func neverCalled() {
` + test.body + "\n}"
			assertSourceBoundaryViolations(t, source, false, test.want)
		})
	}
}

func TestInfrastructureCommandDeclarationShadows(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name: "method receiver shadows import",
			source: `package fixture
import "os/exec"
var _ = exec.Command
type local struct { Command func(string) string }
func (exec local) neverCalled() { _ = exec.Command("incus") }`,
		},
		{
			name: "function type parameter shadows dot import",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
func neverCalled[Command ~string]() { _ = Command("incus") }`,
		},
		{
			name: "signature expression before parameter binding",
			source: `package fixture
import ("os/exec"; "unsafe")
func neverCalled(exec [unsafe.Sizeof(exec.Command("incus"))]byte) { _ = exec }`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "case type expression before type switch binding",
			source: `package fixture
import ("os/exec"; "unsafe")
type local struct { Command func(string) string }
func neverCalled() {
    switch exec := any(local{}).(type) {
    case [unsafe.Sizeof(exec.Command("incus"))]byte: _ = exec
    case local: _ = exec.Command("nft")
    }
}`,
			want: []string{`direct infrastructure command "incus"`},
		},
		{
			name: "type parameter ends at type declaration",
			source: `package fixture
import . "os/exec"
import "unsafe"
type local[Command ~string] [unsafe.Sizeof(string(Command("incus")))]byte
func neverCalled() { _ = Command("nft") }`,
			want: []string{`direct infrastructure command "nft"`},
		},
		{
			name: "receiver type parameter shadows dot import",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
type local[T ~string] struct{}
func (local[Command]) neverCalled() { _ = Command("incus") }`,
		},
		{
			name: "multiple pointer receiver type parameters shadow dot import",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
type local[A, B ~string] struct{}
func (*local[Command, CommandContext]) neverCalled() {
    _ = Command("incus")
    _ = CommandContext("nft")
}`,
		},
		{
			name: "parenthesized receiver type parameter shadows dot import",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
type local[T ~string] struct{}
func ((local[Command])) neverCalled() { _ = Command("incus") }`,
		},
		{
			name: "parenthesized pointer receiver type parameters shadow dot import",
			source: `package fixture
import . "os/exec"
var _ = CommandContext
type local[A, B ~string] struct{}
func ((*(local[Command, CommandContext]))) neverCalled() {
    _ = Command("incus")
    _ = CommandContext("nft")
}`,
		},
		{
			name: "local type conversion shadows dot import",
			source: `package fixture
import . "os/exec"
func neverCalled() {
    { type Command string; _ = Command("incus") }
    _ = Command("nft")
}`,
			want: []string{`direct infrastructure command "nft"`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSourceBoundaryViolations(t, test.source, false, test.want)
		})
	}
}

func assertSourceBoundaryViolations(t *testing.T, source string, providerCommands bool, want []string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	got := sourceBoundaryViolations(fset, file, providerCommands)
	if len(got) != len(want) {
		t.Fatalf("violations = %q; want %q", got, want)
	}
	for i, expected := range want {
		if !strings.Contains(got[i], "fixture.go:") || !strings.Contains(got[i], expected) {
			t.Errorf("violation[%d] = %q; want source position and %q", i, got[i], expected)
		}
	}
}
