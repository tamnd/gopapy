package linter

import "github.com/tamnd/gopapy/parser"

// walkModule calls fn on every Expr reachable from mod's statements
// and their sub-expressions, in pre-order.
func walkModule(mod *parser.Module, fn func(parser.Expr)) {
	for _, s := range mod.Body {
		walkStmtExprs(s, fn)
	}
}

func walkStmtExprs(s parser.Stmt, fn func(parser.Expr)) {
	if s == nil {
		return
	}
	switch n := s.(type) {
	case *parser.ExprStmt:
		walkExpr(n.Value, fn)
	case *parser.Assign:
		walkExpr(n.Value, fn)
		for _, t := range n.Targets {
			walkExpr(t, fn)
		}
	case *parser.AugAssign:
		walkExpr(n.Target, fn)
		walkExpr(n.Value, fn)
	case *parser.AnnAssign:
		walkExpr(n.Target, fn)
		walkExpr(n.Annotation, fn)
		walkExpr(n.Value, fn)
	case *parser.Return:
		walkExpr(n.Value, fn)
	case *parser.Raise:
		walkExpr(n.Exc, fn)
		walkExpr(n.Cause, fn)
	case *parser.Assert:
		walkExpr(n.Test, fn)
		walkExpr(n.Msg, fn)
	case *parser.Delete:
		for _, t := range n.Targets {
			walkExpr(t, fn)
		}
	case *parser.If:
		walkExpr(n.Test, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser.While:
		walkExpr(n.Test, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser.For:
		walkExpr(n.Target, fn)
		walkExpr(n.Iter, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser.AsyncFor:
		walkExpr(n.Target, fn)
		walkExpr(n.Iter, fn)
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
		for _, s2 := range n.Orelse {
			walkStmtExprs(s2, fn)
		}
	case *parser.With:
		for _, item := range n.Items {
			walkExpr(item.ContextExpr, fn)
			walkExpr(item.OptionalVars, fn)
		}
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser.AsyncWith:
		for _, item := range n.Items {
			walkExpr(item.ContextExpr, fn)
			walkExpr(item.OptionalVars, fn)
		}
		for _, s2 := range n.Body {
			walkStmtExprs(s2, fn)
		}
	case *parser.Try:
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
	case *parser.FunctionDef:
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
	case *parser.AsyncFunctionDef:
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
	case *parser.ClassDef:
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
	case *parser.TypeAlias:
		walkExpr(n.Value, fn)
	case *parser.Match:
		walkExpr(n.Subject, fn)
		for _, c := range n.Cases {
			walkExpr(c.Guard, fn)
			for _, s2 := range c.Body {
				walkStmtExprs(s2, fn)
			}
		}
	}
}

func walkArgs(a *parser.Arguments, fn func(parser.Expr)) {
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
func walkExpr(e parser.Expr, fn func(parser.Expr)) {
	if e == nil {
		return
	}
	fn(e)
	switch n := e.(type) {
	case *parser.BoolOp:
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser.BinOp:
		walkExpr(n.Left, fn)
		walkExpr(n.Right, fn)
	case *parser.UnaryOp:
		walkExpr(n.Operand, fn)
	case *parser.Lambda:
		if n.Args != nil {
			walkArgs(n.Args, fn)
		}
		walkExpr(n.Body, fn)
	case *parser.IfExp:
		walkExpr(n.Test, fn)
		walkExpr(n.Body, fn)
		walkExpr(n.OrElse, fn)
	case *parser.Dict:
		for _, k := range n.Keys {
			walkExpr(k, fn)
		}
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser.Set:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser.ListComp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser.SetComp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser.DictComp:
		walkExpr(n.Key, fn)
		walkExpr(n.Value, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser.GeneratorExp:
		walkExpr(n.Elt, fn)
		for _, g := range n.Gens {
			walkExpr(g.Iter, fn)
			walkExpr(g.Target, fn)
			for _, ifx := range g.Ifs {
				walkExpr(ifx, fn)
			}
		}
	case *parser.Await:
		walkExpr(n.Value, fn)
	case *parser.Yield:
		walkExpr(n.Value, fn)
	case *parser.YieldFrom:
		walkExpr(n.Value, fn)
	case *parser.Compare:
		walkExpr(n.Left, fn)
		for _, c := range n.Comparators {
			walkExpr(c, fn)
		}
	case *parser.Call:
		walkExpr(n.Func, fn)
		for _, a := range n.Args {
			walkExpr(a, fn)
		}
		for _, kw := range n.Keywords {
			walkExpr(kw.Value, fn)
		}
	case *parser.FormattedValue:
		walkExpr(n.Value, fn)
		walkExpr(n.FormatSpec, fn)
	case *parser.JoinedStr:
		for _, v := range n.Values {
			walkExpr(v, fn)
		}
	case *parser.TemplateStr:
		for _, interp := range n.Interpolations {
			walkExpr(interp.Value, fn)
			walkExpr(interp.FormatSpec, fn)
		}
	case *parser.Attribute:
		walkExpr(n.Value, fn)
	case *parser.Subscript:
		walkExpr(n.Value, fn)
		walkExpr(n.Slice, fn)
	case *parser.Starred:
		walkExpr(n.Value, fn)
	case *parser.List:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser.Tuple:
		for _, v := range n.Elts {
			walkExpr(v, fn)
		}
	case *parser.Slice:
		walkExpr(n.Lower, fn)
		walkExpr(n.Upper, fn)
		walkExpr(n.Step, fn)
	case *parser.NamedExpr:
		walkExpr(n.Target, fn)
		walkExpr(n.Value, fn)
	}
}
