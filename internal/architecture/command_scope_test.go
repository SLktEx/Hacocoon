package architecture_test

import (
	"go/ast"
	"go/token"
)

type commandShadow struct{ start, end token.Pos }
type commandShadows map[string][]commandShadow

func (shadows commandShadows) add(name string, start, end token.Pos) {
	if _, wanted := shadows[name]; wanted {
		shadows[name] = append(shadows[name], commandShadow{start, end})
	}
}

func (shadows commandShadows) contains(name string, position token.Pos) bool {
	for _, scope := range shadows[name] {
		if scope.start <= position && position < scope.end {
			return true
		}
	}
	return false
}

func (shadows commandShadows) fields(fields *ast.FieldList, start, end token.Pos) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			shadows.add(name.Name, start, end)
		}
	}
}

// Only track declarations that can shadow this file's os/exec import or its
// dot-imported constructors. Scope intervals follow the Go declaration rules;
// no deprecated parser object resolution or package/type loading is involved.
func execImportShadows(file *ast.File, imports map[string]bool) commandShadows {
	shadows := make(commandShadows)
	for name := range imports {
		if name == "." {
			shadows["Command"], shadows["CommandContext"] = nil, nil
		} else {
			shadows[name] = nil
		}
	}
	if len(shadows) == 0 {
		return shadows
	}
	ast.PreorderStack(file, nil, func(node ast.Node, parents []ast.Node) bool {
		shadows.declaration(node, parents)
		return true
	})
	return shadows
}

func (shadows commandShadows) declaration(node ast.Node, parents []ast.Node) {
	switch node := node.(type) {
	case *ast.FuncDecl:
		shadows.fields(node.Recv, node.Type.End(), node.End())
		shadows.fields(node.Type.Params, node.Type.End(), node.End())
		shadows.fields(node.Type.Results, node.Type.End(), node.End())
		shadows.fields(node.Type.TypeParams, node.Name.End(), node.End())
		shadows.receiverTypeParameters(node)
	case *ast.FuncLit:
		shadows.fields(node.Type.Params, node.Type.End(), node.End())
		shadows.fields(node.Type.Results, node.Type.End(), node.End())
	case *ast.ValueSpec:
		for _, name := range node.Names {
			shadows.add(name.Name, node.End(), enclosingScopeEnd(parents))
		}
	case *ast.TypeSpec:
		shadows.add(node.Name.Name, node.Name.Pos(), enclosingScopeEnd(parents))
		shadows.fields(node.TypeParams, node.Name.End(), node.End())
	case *ast.AssignStmt:
		if node.Tok == token.DEFINE && !isTypeSwitchGuard(node, parents) {
			shadows.identifiers(node.Lhs, node.End(), enclosingScopeEnd(parents))
		}
	case *ast.RangeStmt:
		if node.Tok == token.DEFINE {
			shadows.identifiers([]ast.Expr{node.Key, node.Value}, node.Body.Pos(), node.Body.End())
		}
	case *ast.TypeSwitchStmt:
		shadows.typeSwitch(node)
	}
}

func enclosingScopeEnd(parents []ast.Node) token.Pos {
	for i := len(parents) - 1; i >= 0; i-- {
		switch parent := parents[i]; parent.(type) {
		case *ast.File, *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.CaseClause, *ast.CommClause:
			return parent.End()
		}
	}
	return token.NoPos
}

func (shadows commandShadows) identifiers(expressions []ast.Expr, start, end token.Pos) {
	for _, expression := range expressions {
		if name, ok := expression.(*ast.Ident); ok {
			shadows.add(name.Name, start, end)
		}
	}
}

func isTypeSwitchGuard(node *ast.AssignStmt, parents []ast.Node) bool {
	if len(parents) == 0 {
		return false
	}
	parent, ok := parents[len(parents)-1].(*ast.TypeSwitchStmt)
	return ok && parent.Assign == node
}

func (shadows commandShadows) typeSwitch(node *ast.TypeSwitchStmt) {
	assignment, ok := node.Assign.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE {
		return
	}
	for _, statement := range node.Body.List {
		clause := statement.(*ast.CaseClause)
		shadows.identifiers(assignment.Lhs, clause.Colon+1, clause.End())
	}
}

func (shadows commandShadows) receiverTypeParameters(node *ast.FuncDecl) {
	if node.Recv == nil {
		return
	}
	for _, receiver := range node.Recv.List {
		expression := ast.Unparen(receiver.Type)
		if pointer, ok := expression.(*ast.StarExpr); ok {
			expression = ast.Unparen(pointer.X)
		}
		switch expression := expression.(type) {
		case *ast.IndexExpr:
			shadows.identifiers([]ast.Expr{expression.Index}, node.Name.End(), node.End())
		case *ast.IndexListExpr:
			shadows.identifiers(expression.Indices, node.Name.End(), node.End())
		}
	}
}
