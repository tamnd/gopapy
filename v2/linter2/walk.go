package linter2

import "github.com/tamnd/gopapy/v2/parser2"

// walkModule calls fn on every Expr reachable from mod's statements
// and their sub-expressions, in pre-order.
func walkModule(mod *parser2.Module, fn func(parser2.Expr)) {
	for _, s := range mod.Body {
		walkStmtExprs(s, fn)
	}
}

func walkStmtExprs(s parser2.Stmt, fn func(parser2.Expr)) {
	if s == nil {
		return
	}
	switch n := s.(type) {
	case *parser2.ExprStmt:
		walkExpr(n.Value, fn)
	case *parser2.Assign:
		walkExpr(n.Value, fn)
		for _, t := range n.Targets {
			walkExpr(t, fn)
		}
	case *parser2.AugAssign:
		walkExpr(n.Target, fn)
		walkExpr(n.Value, fn)
	case *parser2.AnnAssign:
		walkExpr(n.Target, fn)
		walkExpr(n.Annotation, fn)
		walkExpr(n.Value, fn)
	case *parser2.Return:
		walkExpr(n.Value, fn)
	case *parser2.Raise:
		walkExpr(n.Exc, fn)
		walkExpr(n.Cause, fn)
	case *parser2.Assert:
		walkExpr(n.Test, fn)
		walkExpr(n.Msg, fn)
	case *parser2.Delete:
		for _, t := range n.Targets {
			walkExpr(t, fn)
		}
	case *parser2.If:
		walkExpr(n.Test, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser2.While:
		walkExpr(n.Test, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser2.For:
		walkExpr(n.Target, fn)
		walkExpr(n.Iter, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser2.AsyncFor:
		walkExpr(n.Target, fn)
		walkExpr(n.Iter, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser2.With:
		for _, item := range n.Items {
			walkExpr(item.ContextExpr, fn)
			walkExpr(item.OptionalVars, fn)
		}
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser2.AsyncWith:
		for _, item := range n.Items {
			walkExpr(item.ContextExpr, fn)
			walkExpr(item.OptionalVars, fn)
		}
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser2.Try:
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, h := range n.Handlers {
			walkExpr(h.Type, fn)
			for _, s2 := range h.Body {
				walkStmtExprs(s2, fn)
			}
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Finalbody {
			walkStmtExprs(s2, fn)
		}
	case *parser2.FunctionDef:
		for _, d := range n.DecoratorList {
			walkExpr(d, fn)
		}
		if n.Args != nil {
			walkArgs(n.Args, fn)
		}
		walkExpr(n.Returns, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser2.AsyncFunctionDef:
		for _, d := range n.DecoratorList {
			walkExpr(d, fn)
		}
		if n.Args != nil {
			walkArgs(n.Args, fn)
		}
		walkExpr(n.Returns, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser2.ClassDef:
		for _, d := range n.DecoratorList {
			walkExpr(d, fn)
		}
		for _, b := range n.Bases {
			walkExpr(b, fn)
		}
		for _, kw := range n.Keywords {
			walkExpr(kw.Value, fn)
		}
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser2.TypeAlias:
		walkExpr(n.Value, fn)
	case *parser2.Match:
		walkExpr(n.Subject, fn)
		for _, c := range n.Cases {
			walkExpr(c.Guard, fn)
			for _, s2 := range c.Body {
				walkStmtExprs(s2, fn)
			}
		}
	}
}

func walkArgs(a *parser2.Arguments, fn func(parser2.Expr)) {
	for _, arg := range a.PosOnly {
		walkExpr(arg.Annotation, fn)
	}
	for _, arg := range a.Args {
		walkExpr(arg.Annotation, fn)
	}
	if a.Vararg != nil {
		walkExpr(a.Vararg.Annotation, fn)
	}
	for _, arg := range a.KwOnly {
		walkExpr(arg.Annotation, fn)
	}
	if a.Kwarg != nil {
		walkExpr(a.Kwarg.Annotation, fn)
	}
	for _, d := range a.Defaults {
		walkExpr(d, fn)
	}
	for _, d := range a.KwOnlyDef {
		walkExpr(d, fn)
	}
}

// walkExpr calls fn(e) and then recurses into sub-expressions in pre-order.
func walkExpr(e parser2.Expr, fn func(parser2.Expr)) {
	if e == nil {
		return
	}
	fn(e)
	switch n := e.(type) {
	case *parser2.BoolOp:
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser2.BinOp:
		walkExpr(n.Left, fn)
		walkExpr(n.Right, fn)
	case *parser2.UnaryOp:
		walkExpr(n.Operand, fn)
	case *parser2.Lambda:
		if n.Args != nil {
			walkArgs(n.Args, fn)
		}
		walkExpr(n.Body, fn)
	case *parser2.IfExp:
		walkExpr(n.Test, fn)
		walkExpr(n.Body, fn)
		walkExpr(n.OrElse, fn)
	case *parser2.Dict:
		for _, k := range n.Keys {
			walkExpr(k, fn)
		}
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser2.Set:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser2.ListComp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser2.SetComp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser2.DictComp:
		walkExpr(n.Key, fn)
		walkExpr(n.Value, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser2.GeneratorExp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser2.Await:
		walkExpr(n.Value, fn)
	case *parser2.Yield:
		walkExpr(n.Value, fn)
	case *parser2.YieldFrom:
		walkExpr(n.Value, fn)
	case *parser2.Compare:
		walkExpr(n.Left, fn)
		for _, c := range n.Comparators {
			walkExpr(c, fn)
		}
	case *parser2.Call:
		walkExpr(n.Func, fn)
		for _, a := range n.Args {
			walkExpr(a, fn)
		}
		for _, kw := range n.Keywords {
			walkExpr(kw.Value, fn)
		}
	case *parser2.FormattedValue:
		walkExpr(n.Value, fn)
		walkExpr(n.FormatSpec, fn)
	case *parser2.JoinedStr:
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser2.TemplateStr:
		for _, interp := range n.Interpolations {
			walkExpr(interp.Value, fn)
			walkExpr(interp.FormatSpec, fn)
		}
	case *parser2.Attribute:
		walkExpr(n.Value, fn)
	case *parser2.Subscript:
		walkExpr(n.Value, fn)
		walkExpr(n.Slice, fn)
	case *parser2.Starred:
		walkExpr(n.Value, fn)
	case *parser2.List:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser2.Tuple:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser2.Slice:
		walkExpr(n.Lower, fn)
		walkExpr(n.Upper, fn)
		walkExpr(n.Step, fn)
	case *parser2.NamedExpr:
		walkExpr(n.Target, fn)
		walkExpr(n.Value, fn)
	}
}
