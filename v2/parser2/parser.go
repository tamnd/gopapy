// Package parser2 is gopapy v2's hand-written recursive-descent
// expression parser. It is the parser-swap landing point per
// roadmap v8: v0.1.28 ships literals, names, parenthesized
// expressions, unary operators (+, -, not, ~), and the four
// arithmetic binary operators (+, -, *, /) with correct precedence.
// v0.1.29 grows the surface to the rest of Python's expression
// grammar; v0.1.30 covers f-strings and t-strings; v0.2.0 declares
// v2 as the recommended path.
//
// v2 is self-contained today. v1's `ast` and `lex` packages can't
// be imported until v1's module path is renormalized (the /v1 suffix
// collides with Go's strict major-version rule for v0.x.x tags).
// That convergence is a roadmap-v9 concern.
package parser2

import (
	"fmt"
	"strconv"
)

// ParseExpression parses src as a single Python expression and
// returns the parser2 Expr tree. An error is returned for empty
// input, syntax errors, or constructs outside v0.1.28's coverage.
func ParseExpression(src string) (Expr, error) {
	p, err := newParser(src)
	if err != nil {
		return nil, err
	}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkEOF {
		return nil, fmt.Errorf("%d:%d: unexpected token %s after expression",
			p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
	}
	return expr, nil
}

type parser struct {
	sc  *scanner
	cur token
}

func newParser(src string) (*parser, error) {
	p := &parser{sc: newScanner(src)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *parser) advance() error {
	tok, err := p.sc.next()
	if err != nil {
		return err
	}
	p.cur = tok
	return nil
}

// Precedence climber. Today there are only two binary precedence
// levels in scope (multiplicative > additive); the structure is set
// up so v0.1.29 can grow comparisons, boolean ops, and bit ops by
// adding parseAnd/parseOr/parseCompare layers above and parseShift,
// parseBitwise, etc. below without rewriting the existing layers.
//
// `parseOr` is the entry point because Python's expression hierarchy
// is rooted at boolean-or; today it just delegates to parseAdditive.
func (p *parser) parseOr() (Expr, error) {
	return p.parseAdditive()
}

func (p *parser) parseAdditive() (Expr, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkPlus || p.cur.kind == tkMinus {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: opString(op.kind), Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkStar || p.cur.kind == tkSlash {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: opString(op.kind), Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	switch p.cur.kind {
	case tkPlus, tkMinus, tkTilde:
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryOp{P: op.pos, Op: unaryOpString(op.kind), Operand: operand}, nil
	case tkName:
		// `not` is a name-token at the lex layer; the parser promotes
		// it to unary when seen in unary position.
		if p.cur.val == "not" {
			op := p.cur
			if err := p.advance(); err != nil {
				return nil, err
			}
			operand, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			return &UnaryOp{P: op.pos, Op: "Not", Operand: operand}, nil
		}
	}
	return p.parseAtom()
}

func (p *parser) parseAtom() (Expr, error) {
	tok := p.cur
	switch tok.kind {
	case tkInt:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := strconv.ParseInt(tok.val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%d:%d: invalid int literal %q",
				tok.pos.Line, tok.pos.Col, tok.val)
		}
		return &Constant{P: tok.pos, Kind: "int", Value: v}, nil
	case tkFloat:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := strconv.ParseFloat(tok.val, 64)
		if err != nil {
			return nil, fmt.Errorf("%d:%d: invalid float literal %q",
				tok.pos.Line, tok.pos.Col, tok.val)
		}
		return &Constant{P: tok.pos, Kind: "float", Value: v}, nil
	case tkString:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &Constant{P: tok.pos, Kind: "str", Value: tok.val}, nil
	case tkName:
		if err := p.advance(); err != nil {
			return nil, err
		}
		switch tok.val {
		case "None":
			return &Constant{P: tok.pos, Kind: "None"}, nil
		case "True":
			return &Constant{P: tok.pos, Kind: "True"}, nil
		case "False":
			return &Constant{P: tok.pos, Kind: "False"}, nil
		}
		return &Name{P: tok.pos, Id: tok.val}, nil
	case tkLParen:
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.cur.kind != tkRParen {
			return nil, fmt.Errorf("%d:%d: expected ')', got %s",
				p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return inner, nil
	}
	return nil, fmt.Errorf("%d:%d: unexpected token %s",
		tok.pos.Line, tok.pos.Col, tok.kind)
}

func opString(k tokKind) string {
	switch k {
	case tkPlus:
		return "Add"
	case tkMinus:
		return "Sub"
	case tkStar:
		return "Mult"
	case tkSlash:
		return "Div"
	}
	return k.String()
}

func unaryOpString(k tokKind) string {
	switch k {
	case tkPlus:
		return "UAdd"
	case tkMinus:
		return "USub"
	case tkTilde:
		return "Invert"
	}
	return k.String()
}
