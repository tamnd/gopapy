package linter

import (
	"github.com/tamnd/gopapy/diag"
	"github.com/tamnd/gopapy/parser"
)

func checkF632(mod *parser.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser.Expr) {
		c, ok := e.(*parser.Compare)
		if !ok {
			return
		}
		for i, op := range c.Ops {
			if i >= len(c.Comparators) {
				break
			}
			if op != "Is" && op != "IsNot" {
				continue
			}
			if !isLiteralForIs(c.Comparators[i]) {
				continue
			}
			out = append(out, diag.Diagnostic{
				Pos:      c.P,
				End:      c.P,
				Severity: diag.SeverityWarning,
				Code:     CodeIsWithLiteral,
				Msg:      "use of `is` with a literal, did you mean `==`?",
			})
		}
	})
	return out
}

func isLiteralForIs(e parser.Expr) bool {
	switch v := e.(type) {
	case *parser.Constant:
		return v.Kind != "None"
	case *parser.Tuple:
		return allConstants(v.Elts)
	case *parser.List:
		return allConstants(v.Elts)
	case *parser.Set:
		return allConstants(v.Elts)
	case *parser.Dict:
		return allConstants(v.Keys) && allConstants(v.Values)
	case *parser.UnaryOp:
		if _, ok := v.Operand.(*parser.Constant); ok {
			switch v.Op {
			case "USub", "UAdd", "Invert":
				return true
			}
		}
	case *parser.Name:
		switch v.Id {
		case "Ellipsis", "NotImplemented":
			return true
		}
	}
	return false
}

func allConstants(es []parser.Expr) bool {
	for _, e := range es {
		if e == nil {
			return false
		}
		if _, ok := e.(*parser.Constant); !ok {
			return false
		}
	}
	return true
}
