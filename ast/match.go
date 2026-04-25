package ast

import (
	"github.com/tamnd/gopapy/parser"
)

// emitMatch lowers a parser.MatchStmt into the canonical Match AST node.
// The subject is treated as an assignment-target list so a comma-separated
// `match a, b:` collapses into a Tuple subject. Each case clause becomes
// one MatchCase with its pattern, optional guard, and body.
func emitMatch(m *parser.MatchStmt) StmtNode {
	subject := emitAssignTarget(m.Subject, true)
	cases := make([]*MatchCase, 0, len(m.Cases))
	for _, c := range m.Cases {
		cases = append(cases, &MatchCase{
			Pattern: emitPattern(c.Pattern),
			Guard:   emitExprOpt(c.Guard),
			Body:    emitBlock(c.Body),
		})
	}
	return &Match{Pos: pos(m.Pos), Subject: subject, Cases: cases}
}

func emitPattern(p *parser.Pattern) PatternNode {
	if p == nil {
		return nil
	}
	inner := emitOrPattern(p.Or)
	if p.As != "" {
		// Trailing `as NAME`. The captured pattern moves into the As's
		// pattern slot. `_ as x` would be MatchAs(pattern=None, name=x);
		// we don't see that here because `_` lexes as a Capture which
		// becomes MatchAs(None, None) — so unwrap it back when wrapping.
		if asNode, ok := inner.(*MatchAs); ok && asNode.Name == "" && asNode.Pattern == nil {
			inner = nil
		}
		return &MatchAs{Pos: pos(p.Pos), Pattern: inner, Name: p.As}
	}
	return inner
}

func emitOrPattern(o *parser.OrPattern) PatternNode {
	if len(o.Tail) == 0 {
		return emitClosedPattern(o.Head)
	}
	parts := make([]PatternNode, 0, 1+len(o.Tail))
	parts = append(parts, emitClosedPattern(o.Head))
	for _, c := range o.Tail {
		parts = append(parts, emitClosedPattern(c))
	}
	return &MatchOr{Pos: pos(o.Pos), Patterns: parts}
}

func emitClosedPattern(c *parser.ClosedPattern) PatternNode {
	switch {
	case c.Class != nil:
		return emitClassPattern(c.Class)
	case c.Value != nil:
		return &MatchValue{Pos: pos(c.Pos), Value: emitPatDottedExpr(c.Value.Head, c.Value.Tail, pos(c.Value.Pos))}
	case c.Sequence != nil:
		return emitSeqPattern(c.Sequence)
	case c.Mapping != nil:
		return emitMapPattern(c.Mapping)
	case c.Literal != nil:
		return emitLitPattern(c.Literal)
	case c.Group != nil:
		return emitPattern(c.Group)
	default:
		// CapturePattern: bare NAME. `_` is the wildcard.
		if c.Capture == "_" {
			return &MatchAs{Pos: pos(c.Pos)}
		}
		return &MatchAs{Pos: pos(c.Pos), Name: c.Capture}
	}
}

func emitPatDottedExpr(head string, tail []string, p Pos) ExprNode {
	var cur ExprNode = &Name{Pos: p, Id: head, Ctx: &Load{}}
	for _, seg := range tail {
		cur = &Attribute{Pos: p, Value: cur, Attr: seg, Ctx: &Load{}}
	}
	return cur
}

func emitClassPattern(cp *parser.ClassPattern) PatternNode {
	cls := emitPatDottedExpr(cp.Cls.Head, cp.Cls.Tail, pos(cp.Cls.Pos))
	var posPats []PatternNode
	var kwAttrs []string
	var kwPats []PatternNode
	for _, a := range cp.Args {
		if a.Keyword != "" {
			kwAttrs = append(kwAttrs, a.Keyword)
			kwPats = append(kwPats, emitPattern(a.Value))
		} else {
			posPats = append(posPats, emitPattern(a.Pos1))
		}
	}
	return &MatchClass{
		Pos: pos(cp.Pos), Cls: cls,
		Patterns: posPats, KwdAttrs: kwAttrs, KwdPatterns: kwPats,
	}
}

func emitSeqPattern(s *parser.SeqPattern) PatternNode {
	items := s.Items
	if s.Paren {
		items = s.PItems
	}
	pats := make([]PatternNode, 0, len(items))
	for _, it := range items {
		if it.Star != nil {
			name := it.Star.Name
			if name == "_" {
				name = ""
			}
			pats = append(pats, &MatchStar{Pos: pos(it.Pos), Name: name})
		} else {
			pats = append(pats, emitPattern(it.Pat))
		}
	}
	return &MatchSequence{Pos: pos(s.Pos), Patterns: pats}
}

func emitMapPattern(m *parser.MapPattern) PatternNode {
	var keys []ExprNode
	var pats []PatternNode
	rest := ""
	for _, it := range m.Items {
		if it.Rest != "" {
			rest = it.Rest
			continue
		}
		keys = append(keys, emitMapKey(it.Key))
		pats = append(pats, emitPattern(it.Pattern))
	}
	return &MatchMapping{Pos: pos(m.Pos), Keys: keys, Patterns: pats, Rest: rest}
}

func emitMapKey(k *parser.MapKey) ExprNode {
	switch {
	case k.Number != "":
		c := &Constant{Pos: pos(k.Pos), Value: numberConstant(k.Number)}
		if k.Sign == "-" {
			return &UnaryOp{Pos: pos(k.Pos), Op: &USub{}, Operand: c}
		}
		return c
	case len(k.String) > 0:
		return stringConstant(pos(k.Pos), k.String)
	case k.True:
		return &Constant{Pos: pos(k.Pos), Value: ConstantValue{Kind: ConstantBool, Bool: true}}
	case k.False_:
		return &Constant{Pos: pos(k.Pos), Value: ConstantValue{Kind: ConstantBool, Bool: false}}
	case k.None:
		return &Constant{Pos: pos(k.Pos), Value: ConstantValue{Kind: ConstantNone}}
	case k.Value != nil:
		return emitPatDottedExpr(k.Value.Head, k.Value.Tail, pos(k.Value.Pos))
	}
	return nil
}

func emitLitPattern(l *parser.LitPattern) PatternNode {
	p := pos(l.Pos)
	switch {
	case l.True:
		return &MatchSingleton{Pos: p, Value: ConstantValue{Kind: ConstantBool, Bool: true}}
	case l.False_:
		return &MatchSingleton{Pos: p, Value: ConstantValue{Kind: ConstantBool, Bool: false}}
	case l.None:
		return &MatchSingleton{Pos: p, Value: ConstantValue{Kind: ConstantNone}}
	case len(l.String) > 0:
		return &MatchValue{Pos: p, Value: stringConstant(p, l.String)}
	case l.Number != "":
		var val ExprNode = &Constant{Pos: p, Value: numberConstant(l.Number)}
		if l.Sign == "-" {
			val = &UnaryOp{Pos: p, Op: &USub{}, Operand: val}
		} else if l.Sign == "+" {
			val = &UnaryOp{Pos: p, Op: &UAdd{}, Operand: val}
		}
		if l.Imag != "" {
			imag := &Constant{Pos: p, Value: numberConstant(l.Imag)}
			if l.Op == "-" {
				val = &BinOp{Pos: p, Left: val, Op: &Sub{}, Right: imag}
			} else {
				val = &BinOp{Pos: p, Left: val, Op: &Add{}, Right: imag}
			}
		}
		return &MatchValue{Pos: p, Value: val}
	}
	return nil
}
