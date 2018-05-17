package lint

import (
	"fmt"
	"go/ast"
	"go/token"
)

// ParenthesisChecker detects some cases where parenthesis are unnecessary
type ParenthesisChecker struct {
	ctx *Context
}

// NewParenthesisChecker returns initialized checker for type expressions.
func newParenthesisChecker(ctx *Context) Checker {
	return &ParenthesisChecker{
		ctx: ctx,
	}
}

// Check runs parenthesis checks for f.
//
// Features
//
// Detects parenthesis statements which could be simplified
// and offsers the way how to do it.
func (c *ParenthesisChecker) Check(f *ast.File) {
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Type.Results == nil {
				continue
			}
			for _, res := range decl.Type.Results.List {
				c.validateType(res.Type)
			}
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				continue
			}
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.ValueSpec); ok {
					if spec.Type == nil {
						continue
					}
					c.validateType(spec.Type)
				}
				if spec, ok := spec.(*ast.TypeSpec); ok {
					if spec.Type == nil {
						continue
					}
					c.validateType(spec.Type)
				}
			}
		}
	}
}

func (c *ParenthesisChecker) validateType(n ast.Node) {
	// TODO improve suggestions for complex cases like (func([](func())))
	// TODO improve linter output to write full type, not just place
	// where it could be simplified
	ast.Inspect(n, func(n ast.Node) bool {
		if n, ok := n.(*ast.ArrayType); ok {
			c.validateType(n.Elt)

			if expr, ok := n.Len.(*ast.ParenExpr); ok {
				c.warn(expr)
			}
			return false
		}

		expr, ok := n.(*ast.ParenExpr)
		if !ok {
			return true
		}
		c.warn(expr)
		return false
	})
}

func (c *ParenthesisChecker) warn(expr *ast.ParenExpr) {
	c.ctx.addWarning(Warning{
		Kind: "parenthesis",
		Node: expr,
		Text: fmt.Sprintf("could simplify %s to %s", nodeString(c.ctx.FileSet, expr), nodeString(c.ctx.FileSet, expr.X)),
	})

}