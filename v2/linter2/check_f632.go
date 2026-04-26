package linter2

import (
	"github.com/tamnd/gopapy/v2/diag"
	"github.com/tamnd/gopapy/v2/parser2"
)

func checkF632(mod *parser2.Module) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	var out []diag.Diagnostic
	walkModule(mod, func(e parser2.Expr) {
		c, ok := e.(*parser2.Compare)
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

func isLiteralForIs(e parser2.Expr) bool {
	switch v := e.(type) {
	case *parser2.Constant:
		return v.Kind != "None"
	case *parser2.Tuple:
		return allConstants(v.Elts)
	case *parser2.List:
		return allConstants(v.Elts)
	case *parser2.Set:
		return allConstants(v.Elts)
	case *parser2.Dict:
		return allConstants(v.Keys) && allConstants(v.Values)
	case *parser2.UnaryOp:
		if _, ok := v.Operand.(*parser2.Constant); ok {
			switch v.Op {
			case "USub", "UAdd", "Invert":
				return true
			}
		}
	case *parser2.Name:
		switch v.Id {
		case "Ellipsis", "NotImplemented":
			return true
		}
	}
	return false
}

func allConstants(es []parser2.Expr) bool {
	for _, e := range es {
		if e == nil {
			return false
		}
		if _, ok := e.(*parser2.Constant); !ok {
			return false
		}
	}
	return true
}
