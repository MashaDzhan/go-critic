package checkers

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/go-lintpack/lintpack"
	"github.com/go-lintpack/lintpack/astwalk"
	"github.com/go-toolsmith/astcast"
	"github.com/go-toolsmith/astp"
)

func init() {
	var info lintpack.CheckerInfo
	info.Name = "truncateCmp"
	info.Tags = []string{"diagnostic", "experimental"}
	info.Params = lintpack.CheckerParams{
		"skipArchDependent": {
			Value: true,
			Usage: "whether to skip int/uint/uintptr types",
		},
	}
	info.Summary = "Detects potential truncation issues when comparing ints of different sizes"
	info.Before = `
func f(x int32, y int16) bool {
  return int16(x) < y
}`
	info.After = `
func f(x int32, int16) bool {
  return x < int32(y)
}`

	collection.AddChecker(&info, func(ctx *lintpack.CheckerContext) lintpack.FileWalker {
		c := &truncateCmpChecker{ctx: ctx}
		c.skipArchDependent = info.Params.Bool("skipArchDependent")
		return astwalk.WalkerForExpr(c)
	})
}

type truncateCmpChecker struct {
	astwalk.WalkHandler
	ctx *lintpack.CheckerContext

	skipArchDependent bool
}

func (c *truncateCmpChecker) VisitExpr(expr ast.Expr) {
	cmp := astcast.ToBinaryExpr(expr)
	switch cmp.Op {
	case token.LSS, token.GTR, token.LEQ, token.GEQ, token.EQL, token.NEQ:
		if astp.IsBasicLit(cmp.X) || astp.IsBasicLit(cmp.Y) {
			return // Don't bother about untyped consts
		}
		if !c.checkCmp(cmp.X, cmp.Y) {
			c.checkCmp(cmp.Y, cmp.X)
		}
	default:
		return
	}
}

func (c *truncateCmpChecker) checkCmp(cmpX, cmpY ast.Expr) bool {
	// Check if we have a cast to a type that can truncate.
	xcast := astcast.ToCallExpr(cmpX)
	switch astcast.ToIdent(xcast.Fun).Name {
	case "int8", "int16", "int32", "uint8", "uint16", "uint32":
		// OK.
	default:
		return false
	}
	if len(xcast.Args) != 1 {
		return false // Just in case of the shadowed builtin
	}

	x := xcast.Args[0]
	y := cmpY

	// Check that both x and y are signed or unsigned int-typed.
	xtyp, ok := c.ctx.TypesInfo.TypeOf(x).Underlying().(*types.Basic)
	if !ok || xtyp.Info()&types.IsInteger == 0 {
		return false
	}
	ytyp, ok := c.ctx.TypesInfo.TypeOf(y).Underlying().(*types.Basic)
	if !ok || xtyp.Info() != ytyp.Info() {
		return false
	}

	xsize := c.ctx.SizesInfo.Sizeof(xtyp)
	ysize := c.ctx.SizesInfo.Sizeof(ytyp)
	if xsize <= ysize {
		return false
	}

	if c.skipArchDependent {
		switch xtyp.Kind() {
		case types.Int, types.Uint, types.Uintptr:
			return false
		}
	}

	c.warn(xcast, xsize*8, ysize*8, xtyp.String())
	return true
}

func (c *truncateCmpChecker) warn(cause ast.Expr, xsize, ysize int64, suggest string) {
	c.ctx.Warn(cause, "truncation in comparison %d->%d bit; cast the other operand to %s instead", xsize, ysize, suggest)
}
