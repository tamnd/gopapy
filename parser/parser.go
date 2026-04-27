// Package parser2 is gopapy v2's hand-written recursive-descent
// expression parser. As of v0.1.29 it covers the entire Python
// expression grammar except f-strings and t-strings: literals,
// names, parenthesized expressions, all unary and binary operators
// with correct precedence, comparisons (chained), boolean ops,
// attribute and subscript access, slices, calls (including starred
// and double-starred), collection literals (list/tuple/dict/set),
// comprehensions and generator expressions, lambdas, walrus, and
// the conditional expression `a if b else c`.
//
// Statements arrive in v0.1.30 (which also brings INDENT/DEDENT
// tracking and `ParseFile`). f-strings and t-strings arrive in
// v0.1.31. v0.2.0 declares v2 the recommended path.
//
// v2 is self-contained today. v1's `ast` and `lex` packages can't
// be imported until v1's module path is renormalized (the /v1 suffix
// collides with Go's strict major-version rule for v0.x.x tags).
// That convergence is a roadmap-v9 concern.
package parser

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ParseExpression parses src as a single Python expression and
// returns the parser2 Expr tree. An error is returned for empty
// input, syntax errors, or constructs outside parser2's coverage.
func ParseExpression(src string) (Expr, error) {
	p, err := newParser(src)
	if err != nil {
		return nil, err
	}
	expr, err := p.parseExpr()
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
	sc   *scanner
	cur  token
	peek token
	hasPeek bool
}

func newParser(src string) (*parser, error) {
	p := &parser{sc: newScanner(src)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *parser) advance() error {
	if p.hasPeek {
		p.cur = p.peek
		p.hasPeek = false
		return nil
	}
	tok, err := p.sc.next()
	if err != nil {
		return err
	}
	p.cur = tok
	return nil
}

// peekTok returns the next token without consuming it. Cached so
// repeated peeks at the same position are free.
func (p *parser) peekTok() (token, error) {
	if p.hasPeek {
		return p.peek, nil
	}
	tok, err := p.sc.next()
	if err != nil {
		return token{}, err
	}
	p.peek = tok
	p.hasPeek = true
	return tok, nil
}

func (p *parser) expect(k tokKind) (token, error) {
	if p.cur.kind != k {
		return token{}, fmt.Errorf("%d:%d: expected %s, got %s",
			p.cur.pos.Line, p.cur.pos.Col, k, p.cur.kind)
	}
	tok := p.cur
	if err := p.advance(); err != nil {
		return token{}, err
	}
	return tok, nil
}

// isKeyword reports whether the current token is a name with the
// given keyword text. Python's keywords lex as ordinary names; the
// parser promotes them based on context.
func (p *parser) isKeyword(kw string) bool {
	return p.cur.kind == tkName && p.cur.val == kw
}

// ----- Top-level expression -----

// parseExpr is the top entry. It dispatches lambda / walrus /
// ternary; everything else flows down through the precedence ladder.
func (p *parser) parseExpr() (Expr, error) {
	if p.isKeyword("lambda") {
		return p.parseLambda()
	}
	if p.isKeyword("yield") {
		return p.parseYieldExpr()
	}
	// walrus at top level: NAME ':=' expr
	if p.cur.kind == tkName && !isReservedKeyword(p.cur.val) {
		nxt, err := p.peekTok()
		if err != nil {
			return nil, err
		}
		if nxt.kind == tkWalrus {
			return p.parseNamedExprFromName()
		}
	}
	return p.parseTernary()
}

// parseYieldExpr handles `yield`, `yield expr`, or `yield from expr`.
// Caller has confirmed cur is the `yield` keyword.
func (p *parser) parseYieldExpr() (Expr, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.isKeyword("from") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &YieldFrom{P: pos, Value: v}, nil
	}
	// bare yield: stop at expression-list terminators
	if isYieldTerminator(p.cur.kind) {
		return &Yield{P: pos}, nil
	}
	v, err := p.parseTestlistOrStarExpr()
	if err != nil {
		return nil, err
	}
	return &Yield{P: pos, Value: v}, nil
}

// isYieldTerminator reports kinds that end a bare `yield`.
func isYieldTerminator(k tokKind) bool {
	switch k {
	case tkRParen, tkRBrack, tkRBrace, tkComma, tkColon,
		tkSemi, tkNewline, tkEOF, tkDedent, tkAssign:
		return true
	}
	return false
}

// parseNamedExprFromName consumes `NAME := expr`. cur is the name.
func (p *parser) parseNamedExprFromName() (Expr, error) {
	nameTok := p.cur
	if err := p.advance(); err != nil {
		return nil, err
	}
	// cur is := now
	if err := p.advance(); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &NamedExpr{
		P:      nameTok.pos,
		Target: &Name{P: nameTok.pos, Id: nameTok.val},
		Value:  value,
	}, nil
}

func (p *parser) parseTernary() (Expr, error) {
	body, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.isKeyword("if") {
		return body, nil
	}
	ifPos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	test, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.isKeyword("else") {
		return nil, fmt.Errorf("%d:%d: expected 'else' in conditional expression",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	orelse, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &IfExp{P: ifPos, Test: test, Body: body, OrElse: orelse}, nil
}

// ----- Boolean and comparison layers -----

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	if !p.isKeyword("or") {
		return left, nil
	}
	pos := left.pos()
	values := []Expr{left}
	for p.isKeyword("or") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		values = append(values, next)
	}
	return &BoolOp{P: pos, Op: "Or", Values: values}, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	if !p.isKeyword("and") {
		return left, nil
	}
	pos := left.pos()
	values := []Expr{left}
	for p.isKeyword("and") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		values = append(values, next)
	}
	return &BoolOp{P: pos, Op: "And", Values: values}, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.isKeyword("not") {
		// `not in` is a comparison op; if peek is `in`, defer to
		// parseCompare. But parseCompare expects a left operand
		// already, so a leading `not` here is always unary.
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &UnaryOp{P: pos, Op: "Not", Operand: operand}, nil
	}
	return p.parseCompare()
}

// parseCompare reads `a OP b OP c ...` as a single Compare node.
// `not in` and `is not` are two-token operators handled inline.
func (p *parser) parseCompare() (Expr, error) {
	left, err := p.parseBitOr()
	if err != nil {
		return nil, err
	}
	var ops []string
	var comps []Expr
	for {
		op, ok, err := p.tryComparisonOp()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		right, err := p.parseBitOr()
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
		comps = append(comps, right)
	}
	if len(ops) == 0 {
		return left, nil
	}
	return &Compare{P: left.pos(), Left: left, Ops: ops, Comparators: comps}, nil
}

func (p *parser) tryComparisonOp() (string, bool, error) {
	switch p.cur.kind {
	case tkLt:
		p.advance()
		return "Lt", true, nil
	case tkGt:
		p.advance()
		return "Gt", true, nil
	case tkLe:
		p.advance()
		return "LtE", true, nil
	case tkGe:
		p.advance()
		return "GtE", true, nil
	case tkEqEq:
		p.advance()
		return "Eq", true, nil
	case tkNotEq:
		p.advance()
		return "NotEq", true, nil
	case tkName:
		switch p.cur.val {
		case "in":
			p.advance()
			return "In", true, nil
		case "is":
			p.advance()
			if p.isKeyword("not") {
				p.advance()
				return "IsNot", true, nil
			}
			return "Is", true, nil
		case "not":
			nxt, err := p.peekTok()
			if err != nil {
				return "", false, err
			}
			if nxt.kind == tkName && nxt.val == "in" {
				p.advance() // not
				p.advance() // in
				return "NotIn", true, nil
			}
		}
	}
	return "", false, nil
}

// ----- Bitwise and shift layers -----

func (p *parser) parseBitOr() (Expr, error) {
	left, err := p.parseBitXor()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkPipe {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseBitXor()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: "BitOr", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseBitXor() (Expr, error) {
	left, err := p.parseBitAnd()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkCaret {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseBitAnd()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: "BitXor", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseBitAnd() (Expr, error) {
	left, err := p.parseShift()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkAmp {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseShift()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: "BitAnd", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseShift() (Expr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for p.cur.kind == tkLShift || p.cur.kind == tkRShift {
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		opName := "LShift"
		if op.kind == tkRShift {
			opName = "RShift"
		}
		left = &BinOp{P: op.pos, Op: opName, Left: left, Right: right}
	}
	return left, nil
}

// ----- Arithmetic layers -----

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
		opName := "Add"
		if op.kind == tkMinus {
			opName = "Sub"
		}
		left = &BinOp{P: op.pos, Op: opName, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		var opName string
		switch p.cur.kind {
		case tkStar:
			opName = "Mult"
		case tkSlash:
			opName = "Div"
		case tkDoubleSl:
			opName = "FloorDiv"
		case tkPercent:
			opName = "Mod"
		case tkAt:
			opName = "MatMult"
		default:
			return left, nil
		}
		op := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinOp{P: op.pos, Op: opName, Left: left, Right: right}
	}
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
	}
	if p.isKeyword("await") {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Await{P: pos, Value: operand}, nil
	}
	return p.parsePower()
}

// parsePower implements `**` with right-associativity and the
// CPython-specific oddity that the right operand is parsed at
// unary level (so `2 ** -1` works) while the left is parsed at
// trailer level (so `-2 ** 2` is `-(2 ** 2)`).
func (p *parser) parsePower() (Expr, error) {
	left, err := p.parseTrailer()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkDoubleStar {
		return left, nil
	}
	op := p.cur
	if err := p.advance(); err != nil {
		return nil, err
	}
	right, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	return &BinOp{P: op.pos, Op: "Pow", Left: left, Right: right}, nil
}

// ----- Trailer (attribute, subscript, call) -----

func (p *parser) parseTrailer() (Expr, error) {
	expr, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur.kind {
		case tkDot:
			pos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind != tkName {
				return nil, fmt.Errorf("%d:%d: expected name after '.', got %s",
					p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
			}
			attr := p.cur.val
			if err := p.advance(); err != nil {
				return nil, err
			}
			expr = &Attribute{P: pos, Value: expr, Attr: attr}
		case tkLParen:
			args, kwargs, callPos, err := p.parseCallArgs()
			if err != nil {
				return nil, err
			}
			expr = &Call{P: callPos, Func: expr, Args: args, Keywords: kwargs}
		case tkLBrack:
			pos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			slice, err := p.parseSubscriptBody()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tkRBrack); err != nil {
				return nil, err
			}
			expr = &Subscript{P: pos, Value: expr, Slice: slice}
		default:
			return expr, nil
		}
	}
}

// parseSubscriptBody parses everything between `[` and `]` for a
// subscript trailer. Returns one of: a plain Expr, a Slice, or a
// Tuple of Expr/Slice for advanced indexing.
func (p *parser) parseSubscriptBody() (Expr, error) {
	first, err := p.parseSubscriptItem()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkComma {
		// PEP 646: a single *Ts subscript is wrapped in an implicit Tuple.
		if _, ok := first.(*Starred); ok {
			return &Tuple{P: first.pos(), Elts: []Expr{first}}, nil
		}
		return first, nil
	}
	pos := first.pos()
	elts := []Expr{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkRBrack {
			break
		}
		next, err := p.parseSubscriptItem()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
	}
	return &Tuple{P: pos, Elts: elts}, nil
}

func (p *parser) parseSubscriptItem() (Expr, error) {
	pos := p.cur.pos
	// PEP 646: `*Ts` is valid inside a subscript (e.g. `tuple[*Ts]`).
	if p.cur.kind == tkStar {
		if err := p.advance(); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &Starred{P: pos, Value: val}, nil
	}
	var lower Expr
	if p.cur.kind != tkColon {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		lower = v
	}
	if p.cur.kind != tkColon {
		return lower, nil
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	var upper Expr
	if p.cur.kind != tkColon && p.cur.kind != tkRBrack && p.cur.kind != tkComma {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		upper = v
	}
	var step Expr
	if p.cur.kind == tkColon {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkRBrack && p.cur.kind != tkComma {
			v, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			step = v
		}
	}
	return &Slice{P: pos, Lower: lower, Upper: upper, Step: step}, nil
}

func (p *parser) parseCallArgs() ([]Expr, []*Keyword, Pos, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume (
		return nil, nil, pos, err
	}
	var args []Expr
	var kwargs []*Keyword
	if p.cur.kind == tkRParen {
		if err := p.advance(); err != nil {
			return nil, nil, pos, err
		}
		return args, kwargs, pos, nil
	}
	for {
		switch {
		case p.cur.kind == tkDoubleStar:
			kp := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, nil, pos, err
			}
			v, err := p.parseExpr()
			if err != nil {
				return nil, nil, pos, err
			}
			kwargs = append(kwargs, &Keyword{P: kp, Arg: "", Value: v})
		case p.cur.kind == tkStar:
			sp := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, nil, pos, err
			}
			v, err := p.parseExpr()
			if err != nil {
				return nil, nil, pos, err
			}
			args = append(args, &Starred{P: sp, Value: v})
		case p.cur.kind == tkName && !isReservedKeyword(p.cur.val):
			// Look ahead one token to see if this is `name=value`.
			nxt, err := p.peekTok()
			if err != nil {
				return nil, nil, pos, err
			}
			if nxt.kind == tkAssign {
				name := p.cur
				if err := p.advance(); err != nil {
					return nil, nil, pos, err
				}
				if err := p.advance(); err != nil { // consume =
					return nil, nil, pos, err
				}
				v, err := p.parseExpr()
				if err != nil {
					return nil, nil, pos, err
				}
				kwargs = append(kwargs, &Keyword{P: name.pos, Arg: name.val, Value: v})
			} else {
				v, err := p.parseExpr()
				if err != nil {
					return nil, nil, pos, err
				}
				// genexp: single positional arg followed by `for` in
				// the same paren list.
				if len(args) == 0 && len(kwargs) == 0 && p.isKeyword("for") {
					gens, err := p.parseComprehensionClauses()
					if err != nil {
						return nil, nil, pos, err
					}
					args = append(args, &GeneratorExp{P: v.pos(), Elt: v, Gens: gens})
					if _, err := p.expect(tkRParen); err != nil {
						return nil, nil, pos, err
					}
					return args, kwargs, pos, nil
				}
				args = append(args, v)
			}
		default:
			v, err := p.parseExpr()
			if err != nil {
				return nil, nil, pos, err
			}
			if len(args) == 0 && len(kwargs) == 0 && p.isKeyword("for") {
				gens, err := p.parseComprehensionClauses()
				if err != nil {
					return nil, nil, pos, err
				}
				args = append(args, &GeneratorExp{P: v.pos(), Elt: v, Gens: gens})
				if _, err := p.expect(tkRParen); err != nil {
					return nil, nil, pos, err
				}
				return args, kwargs, pos, nil
			}
			args = append(args, v)
		}
		if p.cur.kind == tkComma {
			if err := p.advance(); err != nil {
				return nil, nil, pos, err
			}
			if p.cur.kind == tkRParen {
				break
			}
			continue
		}
		break
	}
	if _, err := p.expect(tkRParen); err != nil {
		return nil, nil, pos, err
	}
	return args, kwargs, pos, nil
}

// ----- Atom -----

func (p *parser) parseAtom() (Expr, error) {
	tok := p.cur
	switch tok.kind {
	case tkInt:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return parseIntLiteral(tok)
	case tkFloat:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return parseFloatLiteral(tok)
	case tkString, tkFString:
		return p.parseStringAtom()
	case tkEllipsis:
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &Constant{P: tok.pos, Kind: "Ellipsis"}, nil
	case tkName:
		switch tok.val {
		case "None":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: tok.pos, Kind: "None"}, nil
		case "True":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: tok.pos, Kind: "True"}, nil
		case "False":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: tok.pos, Kind: "False"}, nil
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &Name{P: tok.pos, Id: tok.val}, nil
	case tkLParen:
		return p.parseParenAtom()
	case tkLBrack:
		return p.parseListAtom()
	case tkLBrace:
		return p.parseBraceAtom()
	}
	return nil, fmt.Errorf("%d:%d: unexpected token %s",
		tok.pos.Line, tok.pos.Col, tok.kind)
}

// parseStringAtom collects one or more adjacent string-like tokens
// (plain string, f-string, t-string) and produces a single
// Constant, JoinedStr, or TemplateStr depending on what's there.
// Mixing bytes with any other kind, or t-string with anything other
// than another t-string, is rejected (matches CPython).
func (p *parser) parseStringAtom() (Expr, error) {
	startPos := p.cur.pos
	var plainParts []string
	var joined []Expr
	bytesPrefix := false
	uPrefixSeen := false
	hasFOrPlain := false
	hasT := false
	var tParts []*Constant
	var tInterp []*Interpolation
	tStartConst := func() *Constant {
		// Append the buffered plain text as a Constant before an
		// interpolation, even if empty (PEP 750 needs Strings to
		// alternate one-for-one).
		c := &Constant{P: startPos, Kind: "str", Value: strings.Join(plainParts, "")}
		plainParts = plainParts[:0]
		return c
	}
	for p.cur.kind == tkString || p.cur.kind == tkFString {
		tok := p.cur
		if err := p.advance(); err != nil {
			return nil, err
		}
		if tok.kind == tkString {
			val := tok.val
			isBytes := strings.HasPrefix(val, "b:")
			isUnicode := strings.HasPrefix(val, "u:")
			if isBytes {
				val = val[2:]
			} else if isUnicode {
				val = val[2:]
				uPrefixSeen = true
			}
			if hasT {
				return nil, fmt.Errorf("%d:%d: cannot mix t-string with other strings",
					tok.pos.Line, tok.pos.Col)
			}
			if isBytes {
				if hasFOrPlain && !bytesPrefix {
					return nil, fmt.Errorf("%d:%d: cannot mix bytes and str literals",
						tok.pos.Line, tok.pos.Col)
				}
				bytesPrefix = true
			} else {
				if bytesPrefix {
					return nil, fmt.Errorf("%d:%d: cannot mix bytes and str literals",
						tok.pos.Line, tok.pos.Col)
				}
			}
			hasFOrPlain = true
			plainParts = append(plainParts, val)
			continue
		}
		// tkFString
		payload := tok.fpayload
		if payload == nil {
			return nil, fmt.Errorf("%d:%d: empty f-string payload", tok.pos.Line, tok.pos.Col)
		}
		if bytesPrefix {
			return nil, fmt.Errorf("%d:%d: cannot mix bytes and f/t-string literals",
				tok.pos.Line, tok.pos.Col)
		}
		if payload.template {
			if hasFOrPlain {
				return nil, fmt.Errorf("%d:%d: cannot mix t-string with other strings",
					tok.pos.Line, tok.pos.Col)
			}
			hasT = true
			// Convert this f-string payload into TemplateStr parts.
			// Strings has length len(interps)+1 — start with an empty
			// constant if the first segment is an interpolation.
			pendingText := ""
			pendingPos := tok.pos
			needConstant := true
			for _, seg := range payload.segments {
				if !seg.isInterp {
					if needConstant {
						pendingPos = seg.pos
					}
					pendingText += seg.text
					needConstant = false
					continue
				}
				tParts = append(tParts, &Constant{P: pendingPos, Kind: "str", Value: pendingText})
				pendingText = ""
				needConstant = true
				ip, err := p.buildInterpolation(seg)
				if err != nil {
					return nil, err
				}
				tInterp = append(tInterp, ip)
			}
			tParts = append(tParts, &Constant{P: pendingPos, Kind: "str", Value: pendingText})
			continue
		}
		if hasT {
			return nil, fmt.Errorf("%d:%d: cannot mix t-string with f-string", tok.pos.Line, tok.pos.Col)
		}
		hasFOrPlain = true
		// f-string: lower segments into JoinedStr values, mixing in
		// any buffered plain text first.
		if len(joined) == 0 && len(plainParts) > 0 {
			joined = append(joined, &Constant{P: startPos, Kind: "str", Value: strings.Join(plainParts, "")})
			plainParts = plainParts[:0]
		} else if len(plainParts) > 0 {
			joined = append(joined, &Constant{P: tok.pos, Kind: "str", Value: strings.Join(plainParts, "")})
			plainParts = plainParts[:0]
		}
		for _, seg := range payload.segments {
			if !seg.isInterp {
				if seg.text == "" {
					continue
				}
				joined = append(joined, &Constant{P: seg.pos, Kind: "str", Value: seg.text})
				continue
			}
			fv, err := p.buildFormattedValue(seg)
			if err != nil {
				return nil, err
			}
			// PEP 701: self-documenting expression f"{x=}".
			// Inject (or merge) the source text as a Constant before fv.
			if seg.selfDocText != "" {
				if len(joined) > 0 {
					if c, ok := joined[len(joined)-1].(*Constant); ok && c.Kind == "str" {
						c.Value = c.Value.(string) + seg.selfDocText
					} else {
						joined = append(joined, &Constant{P: seg.pos, Kind: "str", Value: seg.selfDocText})
					}
				} else {
					joined = append(joined, &Constant{P: seg.pos, Kind: "str", Value: seg.selfDocText})
				}
				// Default conversion for '=' is repr unless explicit or format spec present.
				if seg.convert == -1 && seg.spec == nil {
					fv.Conversion = 114
				}
			}
			joined = append(joined, fv)
		}
	}
	if hasT {
		// Any trailing buffered text is already in tParts thanks to
		// the per-segment flush.
		_ = tStartConst
		return &TemplateStr{P: startPos, Strings: tParts, Interpolations: tInterp}, nil
	}
	if len(joined) > 0 {
		// Any remaining plain text (from a trailing plain-string
		// literal, e.g. `f"a" "b"`) becomes a final Constant.
		if len(plainParts) > 0 {
			joined = append(joined, &Constant{P: startPos, Kind: "str", Value: strings.Join(plainParts, "")})
			plainParts = plainParts[:0]
		}
		return &JoinedStr{P: startPos, Values: joined}, nil
	}
	// Empty f-string (no segments) becomes JoinedStr() to match CPython.
	if hasFOrPlain && !bytesPrefix && len(plainParts) == 0 {
		return &JoinedStr{P: startPos, Values: nil}, nil
	}
	kind := "str"
	if bytesPrefix {
		kind = "bytes"
	} else if uPrefixSeen {
		kind = "u"
	}
	return &Constant{P: startPos, Kind: kind, Value: strings.Join(plainParts, "")}, nil
}

// buildFormattedValue parses a single f-string interpolation segment
// into a FormattedValue node, recursively building the format spec
// if present.
func (p *parser) buildFormattedValue(seg fstringSegment) (*FormattedValue, error) {
	val, err := parseInterpExpr(seg.exprSrc, seg.pos)
	if err != nil {
		return nil, err
	}
	var spec Expr
	if seg.spec != nil {
		s, err := p.buildSpecJoined(seg.spec, seg.pos)
		if err != nil {
			return nil, err
		}
		spec = s
	}
	return &FormattedValue{
		P:          seg.pos,
		Value:      val,
		Conversion: seg.convert,
		FormatSpec: spec,
	}, nil
}

// buildInterpolation is the t-string analogue of buildFormattedValue.
func (p *parser) buildInterpolation(seg fstringSegment) (*Interpolation, error) {
	val, err := parseInterpExpr(seg.exprSrc, seg.pos)
	if err != nil {
		return nil, err
	}
	var spec Expr
	if seg.spec != nil {
		s, err := p.buildSpecJoined(seg.spec, seg.pos)
		if err != nil {
			return nil, err
		}
		spec = s
	}
	return &Interpolation{
		P:          seg.pos,
		Value:      val,
		Str:        seg.exprSrc,
		Conversion: seg.convert,
		FormatSpec: spec,
	}, nil
}

// buildSpecJoined turns a parsed format-spec payload into a
// JoinedStr node, recursively. Empty specs become a JoinedStr with
// no values, matching CPython's representation.
func (p *parser) buildSpecJoined(payload *fstringPayload, at Pos) (*JoinedStr, error) {
	var values []Expr
	for _, seg := range payload.segments {
		if !seg.isInterp {
			if seg.text == "" {
				continue
			}
			values = append(values, &Constant{P: seg.pos, Kind: "str", Value: seg.text})
			continue
		}
		fv, err := p.buildFormattedValue(seg)
		if err != nil {
			return nil, err
		}
		values = append(values, fv)
	}
	return &JoinedStr{P: at, Values: values}, nil
}

// parseInterpExpr parses the expression source captured from an
// f-string interpolation. It runs an expression-mode sub-parser, so
// the main scanner state stays untouched.
func parseInterpExpr(src string, at Pos) (Expr, error) {
	if src == "" {
		return nil, fmt.Errorf("%d:%d: empty expression in f-string", at.Line, at.Col)
	}
	sub, err := newParser(src)
	if err != nil {
		return nil, err
	}
	e, err := sub.parseExpr()
	if err != nil {
		return nil, err
	}
	if sub.cur.kind != tkEOF {
		return nil, fmt.Errorf("%d:%d: trailing tokens in f-string expression %q",
			at.Line, at.Col, src)
	}
	return e, nil
}

func parseIntLiteral(tok token) (Expr, error) {
	val := strings.ReplaceAll(tok.val, "_", "")
	var base int
	var digits string
	switch {
	case strings.HasPrefix(val, "0x") || strings.HasPrefix(val, "0X"):
		base, digits = 16, val[2:]
	case strings.HasPrefix(val, "0o") || strings.HasPrefix(val, "0O"):
		base, digits = 8, val[2:]
	case strings.HasPrefix(val, "0b") || strings.HasPrefix(val, "0B"):
		base, digits = 2, val[2:]
	default:
		base, digits = 10, val
	}
	v64, err := strconv.ParseInt(digits, base, 64)
	if err == nil {
		return &Constant{P: tok.pos, Kind: "int", Value: v64}, nil
	}
	if errors.Is(err, strconv.ErrRange) {
		bi := new(big.Int)
		if _, ok := bi.SetString(digits, base); !ok {
			return nil, fmt.Errorf("%d:%d: invalid int literal %q",
				tok.pos.Line, tok.pos.Col, tok.val)
		}
		return &Constant{P: tok.pos, Kind: "int", Value: bi}, nil
	}
	return nil, fmt.Errorf("%d:%d: invalid int literal %q",
		tok.pos.Line, tok.pos.Col, tok.val)
}

func parseFloatLiteral(tok token) (Expr, error) {
	val := strings.ReplaceAll(tok.val, "_", "")
	if strings.HasSuffix(val, "j") || strings.HasSuffix(val, "J") {
		// Complex: keep as-is; adConstValue will parse the imaginary part.
		return &Constant{P: tok.pos, Kind: "complex", Value: val}, nil
	}
	v, err := strconv.ParseFloat(val, 64)
	// ErrRange means overflow (+Inf) or underflow (0.0) — both valid Python values.
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return nil, fmt.Errorf("%d:%d: invalid float literal %q",
			tok.pos.Line, tok.pos.Col, tok.val)
	}
	return &Constant{P: tok.pos, Kind: "float", Value: v}, nil
}

// parseParenAtom handles:
//   ()           -> empty tuple
//   (e)          -> parenthesized expression
//   (e,)         -> 1-tuple
//   (e, f, ...)  -> tuple
//   (e for ...)  -> generator expression
//   (NAME := e)  -> walrus
//   (*e, ...)    -> tuple with starred element
func (p *parser) parseParenAtom() (Expr, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume (
		return nil, err
	}
	if p.cur.kind == tkRParen {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &Tuple{P: pos, Elts: nil}, nil
	}
	first, err := p.parseStarredOrExpr()
	if err != nil {
		return nil, err
	}
	if p.isKeyword("for") {
		gens, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tkRParen); err != nil {
			return nil, err
		}
		return &GeneratorExp{P: pos, Elt: first, Gens: gens}, nil
	}
	if p.cur.kind == tkRParen {
		if err := p.advance(); err != nil {
			return nil, err
		}
		// `(*x)` is illegal in Python, but parsing it as a parenthesized
		// Starred is fine — semantic check is downstream.
		return first, nil
	}
	if _, err := p.expect(tkComma); err != nil {
		return nil, err
	}
	elts := []Expr{first}
	for p.cur.kind != tkRParen {
		next, err := p.parseStarredOrExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
		if p.cur.kind == tkComma {
			if err := p.advance(); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	if _, err := p.expect(tkRParen); err != nil {
		return nil, err
	}
	return &Tuple{P: pos, Elts: elts}, nil
}

func (p *parser) parseListAtom() (Expr, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume [
		return nil, err
	}
	if p.cur.kind == tkRBrack {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &List{P: pos, Elts: nil}, nil
	}
	first, err := p.parseStarredOrExpr()
	if err != nil {
		return nil, err
	}
	if p.isKeyword("for") {
		gens, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tkRBrack); err != nil {
			return nil, err
		}
		return &ListComp{P: pos, Elt: first, Gens: gens}, nil
	}
	elts := []Expr{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkRBrack {
			break
		}
		next, err := p.parseStarredOrExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
	}
	if _, err := p.expect(tkRBrack); err != nil {
		return nil, err
	}
	return &List{P: pos, Elts: elts}, nil
}

func (p *parser) parseBraceAtom() (Expr, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume {
		return nil, err
	}
	if p.cur.kind == tkRBrace {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &Dict{P: pos, Keys: nil, Values: nil}, nil
	}
	// Special leading **other → dict
	if p.cur.kind == tkDoubleStar {
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		keys := []Expr{nil}
		values := []Expr{v}
		for p.cur.kind == tkComma {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind == tkRBrace {
				break
			}
			if p.cur.kind == tkDoubleStar {
				if err := p.advance(); err != nil {
					return nil, err
				}
				vv, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				keys = append(keys, nil)
				values = append(values, vv)
				continue
			}
			k, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tkColon); err != nil {
				return nil, err
			}
			vv, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			keys = append(keys, k)
			values = append(values, vv)
		}
		if _, err := p.expect(tkRBrace); err != nil {
			return nil, err
		}
		return &Dict{P: pos, Keys: keys, Values: values}, nil
	}
	first, err := p.parseStarredOrExpr()
	if err != nil {
		return nil, err
	}
	if p.cur.kind == tkColon {
		// dict literal or dict comprehension
		if err := p.advance(); err != nil {
			return nil, err
		}
		firstVal, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.isKeyword("for") {
			gens, err := p.parseComprehensionClauses()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tkRBrace); err != nil {
				return nil, err
			}
			return &DictComp{P: pos, Key: first, Value: firstVal, Gens: gens}, nil
		}
		keys := []Expr{first}
		values := []Expr{firstVal}
		for p.cur.kind == tkComma {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind == tkRBrace {
				break
			}
			if p.cur.kind == tkDoubleStar {
				if err := p.advance(); err != nil {
					return nil, err
				}
				v, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				keys = append(keys, nil)
				values = append(values, v)
				continue
			}
			k, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tkColon); err != nil {
				return nil, err
			}
			v, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			keys = append(keys, k)
			values = append(values, v)
		}
		if _, err := p.expect(tkRBrace); err != nil {
			return nil, err
		}
		return &Dict{P: pos, Keys: keys, Values: values}, nil
	}
	if p.isKeyword("for") {
		gens, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tkRBrace); err != nil {
			return nil, err
		}
		return &SetComp{P: pos, Elt: first, Gens: gens}, nil
	}
	// set literal
	elts := []Expr{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkRBrace {
			break
		}
		next, err := p.parseStarredOrExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
	}
	if _, err := p.expect(tkRBrace); err != nil {
		return nil, err
	}
	return &Set{P: pos, Elts: elts}, nil
}

// parseStarredOrExpr is the entry for a single element inside a
// collection literal or call args. It returns either Starred(expr)
// or a plain expression.
func (p *parser) parseStarredOrExpr() (Expr, error) {
	if p.cur.kind == tkStar {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &Starred{P: pos, Value: v}, nil
	}
	return p.parseExpr()
}

// ----- Comprehensions -----

func (p *parser) parseComprehensionClauses() ([]*Comprehension, error) {
	var gens []*Comprehension
	for p.isKeyword("for") || (p.isKeyword("async") && p.peekIsFor()) {
		var isAsync bool
		if p.isKeyword("async") {
			isAsync = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		if !p.isKeyword("for") {
			return nil, fmt.Errorf("%d:%d: expected 'for' in comprehension",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		target, err := p.parseTargetList()
		if err != nil {
			return nil, err
		}
		if !p.isKeyword("in") {
			return nil, fmt.Errorf("%d:%d: expected 'in' in comprehension",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		iter, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		var ifs []Expr
		for p.isKeyword("if") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			cond, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			ifs = append(ifs, cond)
		}
		gens = append(gens, &Comprehension{
			Target: target, Iter: iter, Ifs: ifs, IsAsync: isAsync,
		})
	}
	return gens, nil
}

func (p *parser) peekIsFor() bool {
	nxt, err := p.peekTok()
	if err != nil {
		return false
	}
	return nxt.kind == tkName && nxt.val == "for"
}

// parseTargetList parses a comprehension or for-loop target. Allows
// names, starred names, and tuples thereof (without enclosing
// parens). Stops at `in`.
func (p *parser) parseTargetList() (Expr, error) {
	first, err := p.parseTargetAtom()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkComma {
		return first, nil
	}
	pos := first.pos()
	elts := []Expr{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		// trailing comma: peek for `in` or close-bracket
		if p.isKeyword("in") || p.cur.kind == tkRParen ||
			p.cur.kind == tkRBrack || p.cur.kind == tkRBrace {
			break
		}
		next, err := p.parseTargetAtom()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
	}
	return &Tuple{P: pos, Elts: elts}, nil
}

func (p *parser) parseTargetAtom() (Expr, error) {
	if p.cur.kind == tkStar {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		v, err := p.parseTargetAtom()
		if err != nil {
			return nil, err
		}
		return &Starred{P: pos, Value: v}, nil
	}
	if p.cur.kind == tkLParen {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseTargetList()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tkRParen); err != nil {
			return nil, err
		}
		// re-wrap in Tuple if it's a single element to preserve parens
		_ = pos
		return inner, nil
	}
	if p.cur.kind == tkLBrack {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		var elts []Expr
		for p.cur.kind != tkRBrack {
			e, err := p.parseTargetAtom()
			if err != nil {
				return nil, err
			}
			elts = append(elts, e)
			if p.cur.kind == tkComma {
				p.advance()
				continue
			}
			break
		}
		if _, err := p.expect(tkRBrack); err != nil {
			return nil, err
		}
		return &List{P: pos, Elts: elts}, nil
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected target, got %s",
			p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
	}
	tok := p.cur
	if err := p.advance(); err != nil {
		return nil, err
	}
	// allow attribute / subscript targets for completeness
	expr := Expr(&Name{P: tok.pos, Id: tok.val})
	for p.cur.kind == tkDot || p.cur.kind == tkLBrack {
		if p.cur.kind == tkDot {
			pos := p.cur.pos
			p.advance()
			if p.cur.kind != tkName {
				return nil, fmt.Errorf("%d:%d: expected name after '.'",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			attr := p.cur.val
			p.advance()
			expr = &Attribute{P: pos, Value: expr, Attr: attr}
		} else {
			pos := p.cur.pos
			p.advance()
			s, err := p.parseSubscriptBody()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tkRBrack); err != nil {
				return nil, err
			}
			expr = &Subscript{P: pos, Value: expr, Slice: s}
		}
	}
	return expr, nil
}

// ----- Lambda -----

func (p *parser) parseLambda() (Expr, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume 'lambda'
		return nil, err
	}
	args, err := p.parseLambdaArgs()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tkColon); err != nil {
		return nil, err
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &Lambda{P: pos, Args: args, Body: body}, nil
}

func (p *parser) parseLambdaArgs() (*Arguments, error) {
	a := &Arguments{}
	if p.cur.kind == tkColon {
		return a, nil
	}
	state := "pos" // pos | kwonly
	for p.cur.kind != tkColon {
		switch p.cur.kind {
		case tkStar:
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind == tkComma || p.cur.kind == tkColon {
				state = "kwonly"
			} else {
				if p.cur.kind != tkName {
					return nil, fmt.Errorf("%d:%d: expected name after '*'",
						p.cur.pos.Line, p.cur.pos.Col)
				}
				name := p.cur
				p.advance()
				a.Vararg = &Arg{P: name.pos, Name: name.val}
				state = "kwonly"
			}
		case tkDoubleStar:
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind != tkName {
				return nil, fmt.Errorf("%d:%d: expected name after '**'",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			name := p.cur
			p.advance()
			a.Kwarg = &Arg{P: name.pos, Name: name.val}
		case tkSlash:
			// posonly marker — promote current Args to PosOnly
			if err := p.advance(); err != nil {
				return nil, err
			}
			a.PosOnly = a.Args
			a.Args = nil
		case tkName:
			name := p.cur
			p.advance()
			arg := &Arg{P: name.pos, Name: name.val}
			var defaultVal Expr
			if p.cur.kind == tkAssign {
				p.advance()
				v, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				defaultVal = v
			}
			if state == "pos" {
				a.Args = append(a.Args, arg)
				if defaultVal != nil {
					a.Defaults = append(a.Defaults, defaultVal)
				}
			} else {
				a.KwOnly = append(a.KwOnly, arg)
				a.KwOnlyDef = append(a.KwOnlyDef, defaultVal)
			}
		default:
			return nil, fmt.Errorf("%d:%d: unexpected token %s in lambda args",
				p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
		}
		if p.cur.kind == tkComma {
			p.advance()
		} else if p.cur.kind != tkColon {
			return nil, fmt.Errorf("%d:%d: expected ',' or ':' in lambda args, got %s",
				p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
		}
	}
	return a, nil
}

// ----- Helpers -----

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

// isReservedKeyword reports whether a name token text is a Python
// keyword that the parser handles specially (so it should not be
// treated as a plain Name in lookahead checks).
func isReservedKeyword(s string) bool {
	switch s {
	case "and", "or", "not", "in", "is",
		"if", "else", "elif", "for", "while",
		"lambda", "True", "False", "None",
		"def", "class", "return", "yield",
		"import", "from", "as", "with",
		"try", "except", "finally", "raise",
		"pass", "break", "continue",
		"global", "nonlocal", "assert", "del",
		"async", "await", "match", "case":
		return true
	}
	return false
}
