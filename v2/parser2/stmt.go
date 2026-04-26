package parser2

import (
	"fmt"
	"strings"
)

// ParseFile parses src as a complete Python module and returns its
// Module AST. filename is used only for error messages; the bytes
// themselves come from src. f-strings and t-strings are not yet
// supported (v0.1.31); a file containing them returns a position-
// tagged error instead of mis-parsing.
func ParseFile(filename, src string) (*Module, error) {
	p, err := newStmtParser(src)
	if err != nil {
		return nil, prefixErr(err, filename)
	}
	mod, err := p.parseModule()
	if err != nil {
		return nil, prefixErr(err, filename)
	}
	return mod, nil
}

// ParseString is an alias for ParseFile kept for symmetry with v1.
// New code should prefer ParseFile.
func ParseString(filename, src string) (*Module, error) {
	return ParseFile(filename, src)
}

func prefixErr(err error, filename string) error {
	if err == nil || filename == "" {
		return err
	}
	return fmt.Errorf("%s:%v", filename, err)
}

func newStmtParser(src string) (*parser, error) {
	p := &parser{sc: newStmtScanner(src)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return p, nil
}

// ----- Module / block / statement entry points -----

func (p *parser) parseModule() (*Module, error) {
	mod := &Module{}
	for p.cur.kind != tkEOF {
		// Skip stray NEWLINEs at the top level (blank logical lines
		// the lexer collapses are already gone, but a leading or
		// trailing NEWLINE may still arrive when the file is otherwise
		// empty).
		if p.cur.kind == tkNewline {
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		stmts, err := p.parseStatementLine()
		if err != nil {
			return nil, err
		}
		mod.Body = append(mod.Body, stmts...)
	}
	return mod, nil
}

// parseStatementLine consumes one logical line. That line is either a
// compound statement (one Stmt, with its own block) or one or more
// simple statements separated by `;` and terminated by NEWLINE.
func (p *parser) parseStatementLine() ([]Stmt, error) {
	if p.cur.kind == tkAt || isCompoundKeyword(p.cur) {
		s, err := p.parseCompoundStmt()
		if err != nil {
			return nil, err
		}
		return []Stmt{s}, nil
	}
	return p.parseSimpleStmtList()
}

// parseSimpleStmtList reads one or more semicolon-separated small
// statements terminated by NEWLINE (or EOF).
func (p *parser) parseSimpleStmtList() ([]Stmt, error) {
	var stmts []Stmt
	for {
		s, err := p.parseSimpleStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, s)
		if p.cur.kind != tkSemi {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkNewline || p.cur.kind == tkEOF {
			break
		}
	}
	if p.cur.kind == tkNewline {
		if err := p.advance(); err != nil {
			return nil, err
		}
	} else if p.cur.kind != tkEOF && p.cur.kind != tkDedent {
		return nil, fmt.Errorf("%d:%d: expected newline after statement, got %s",
			p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
	}
	return stmts, nil
}

// parseSimpleStmt parses one small statement: keyword form or
// expression-based (assignment / aug-assign / ann-assign / bare expr).
func (p *parser) parseSimpleStmt() (Stmt, error) {
	if p.cur.kind == tkName {
		switch p.cur.val {
		case "pass":
			pos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Pass{P: pos}, nil
		case "break":
			pos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Break{P: pos}, nil
		case "continue":
			pos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Continue{P: pos}, nil
		case "return":
			return p.parseReturnStmt()
		case "raise":
			return p.parseRaiseStmt()
		case "import":
			return p.parseImportStmt()
		case "from":
			return p.parseFromImportStmt()
		case "global":
			return p.parseGlobalNonlocal(true)
		case "nonlocal":
			return p.parseGlobalNonlocal(false)
		case "del":
			return p.parseDeleteStmt()
		case "assert":
			return p.parseAssertStmt()
		}
	}
	return p.parseExprBasedStmt()
}

// parseExprBasedStmt handles bare expression, assignment chain, aug-
// assign, and annotated assignment by first parsing an expression-
// like LHS, then deciding what statement form to build.
func (p *parser) parseExprBasedStmt() (Stmt, error) {
	pos := p.cur.pos
	lhs, err := p.parseTestlistOrStarExpr()
	if err != nil {
		return nil, err
	}
	// Annotated assignment: `target: ann [= value]`.
	if p.cur.kind == tkColon {
		if err := p.advance(); err != nil {
			return nil, err
		}
		ann, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		var value Expr
		if p.cur.kind == tkAssign {
			if err := p.advance(); err != nil {
				return nil, err
			}
			value, err = p.parseTestlistOrStarExpr()
			if err != nil {
				return nil, err
			}
		}
		if err := validateAssignTarget(lhs); err != nil {
			return nil, err
		}
		_, simple := lhs.(*Name)
		return &AnnAssign{P: pos, Target: lhs, Annotation: ann, Value: value, Simple: simple}, nil
	}
	// Augmented assignment.
	if op, ok := augAssignOp(p.cur.kind); ok {
		if err := p.advance(); err != nil {
			return nil, err
		}
		rhs, err := p.parseTestlistOrStarExpr()
		if err != nil {
			return nil, err
		}
		if err := validateAssignTarget(lhs); err != nil {
			return nil, err
		}
		return &AugAssign{P: pos, Target: lhs, Op: op, Value: rhs}, nil
	}
	// Plain assignment: chains via repeated `=`.
	if p.cur.kind == tkAssign {
		targets := []Expr{lhs}
		var value Expr
		for p.cur.kind == tkAssign {
			if err := p.advance(); err != nil {
				return nil, err
			}
			rhs, err := p.parseTestlistOrStarExpr()
			if err != nil {
				return nil, err
			}
			if p.cur.kind == tkAssign {
				targets = append(targets, rhs)
				continue
			}
			value = rhs
			break
		}
		for _, t := range targets {
			if err := validateAssignTarget(t); err != nil {
				return nil, err
			}
		}
		return &Assign{P: pos, Targets: targets, Value: value}, nil
	}
	// Bare expression statement.
	return &ExprStmt{P: pos, Value: lhs}, nil
}

// parseTestlistOrStarExpr parses `expr` or `expr, expr, ...` — a
// possibly-tuple expression as found on the RHS of assignments and the
// targets of for-loops. Trailing comma forces a one-element tuple.
func (p *parser) parseTestlistOrStarExpr() (Expr, error) {
	first, err := p.parseStarOrExprStmt()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkComma {
		return first, nil
	}
	pos := first.pos()
	elts := []Expr{first}
	for p.cur.kind == tkComma {
		// Lookahead: a comma followed by `=`, `:`, augmented assign,
		// `;`, NEWLINE, or end-of-line marks a trailing comma; the
		// tuple is what we have.
		if err := p.advance(); err != nil {
			return nil, err
		}
		if isExprListTerminator(p.cur.kind) {
			break
		}
		next, err := p.parseStarOrExprStmt()
		if err != nil {
			return nil, err
		}
		elts = append(elts, next)
	}
	return &Tuple{P: pos, Elts: elts}, nil
}

func (p *parser) parseStarOrExprStmt() (Expr, error) {
	if p.cur.kind == tkStar {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &Starred{P: pos, Value: val}, nil
	}
	return p.parseExpr()
}

func isExprListTerminator(k tokKind) bool {
	switch k {
	case tkAssign, tkColon, tkSemi, tkNewline, tkEOF, tkDedent,
		tkRParen, tkRBrack, tkRBrace,
		tkPlusAssign, tkMinusAssign, tkStarAssign, tkSlashAssign,
		tkDoubleSlAssign, tkPercentAssign, tkDoubleStarAssign,
		tkAmpAssign, tkPipeAssign, tkCaretAssign,
		tkLShiftAssign, tkRShiftAssign, tkAtAssign:
		return true
	}
	return false
}

func augAssignOp(k tokKind) (string, bool) {
	switch k {
	case tkPlusAssign:
		return "Add", true
	case tkMinusAssign:
		return "Sub", true
	case tkStarAssign:
		return "Mult", true
	case tkSlashAssign:
		return "Div", true
	case tkDoubleSlAssign:
		return "FloorDiv", true
	case tkPercentAssign:
		return "Mod", true
	case tkDoubleStarAssign:
		return "Pow", true
	case tkAmpAssign:
		return "BitAnd", true
	case tkPipeAssign:
		return "BitOr", true
	case tkCaretAssign:
		return "BitXor", true
	case tkLShiftAssign:
		return "LShift", true
	case tkRShiftAssign:
		return "RShift", true
	case tkAtAssign:
		return "MatMult", true
	}
	return "", false
}

// validateAssignTarget rejects expressions that can't appear on the
// LHS of `=`. The check is deliberately permissive — symbols.Build
// catches the deeper cases (assigning to a function call result,
// etc.); the parser only blocks shapes that produce nonsense AST.
func validateAssignTarget(e Expr) error {
	switch n := e.(type) {
	case *Name, *Attribute, *Subscript, *Starred:
		return nil
	case *Tuple:
		for _, el := range n.Elts {
			if err := validateAssignTarget(el); err != nil {
				return err
			}
		}
		return nil
	case *List:
		for _, el := range n.Elts {
			if err := validateAssignTarget(el); err != nil {
				return err
			}
		}
		return nil
	case *Constant:
		return fmt.Errorf("%d:%d: cannot assign to literal", n.P.Line, n.P.Col)
	}
	return fmt.Errorf("%d:%d: cannot assign to %T", e.pos().Line, e.pos().Col, e)
}

// ----- Keyword-led simple statements -----

func (p *parser) parseReturnStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if isStmtEnd(p.cur.kind) {
		return &Return{P: pos}, nil
	}
	val, err := p.parseTestlistOrStarExpr()
	if err != nil {
		return nil, err
	}
	return &Return{P: pos, Value: val}, nil
}

func (p *parser) parseRaiseStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if isStmtEnd(p.cur.kind) {
		return &Raise{P: pos}, nil
	}
	exc, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var cause Expr
	if p.isKeyword("from") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		cause, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	return &Raise{P: pos, Exc: exc, Cause: cause}, nil
}

func (p *parser) parseImportStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	first, err := p.parseDottedAlias()
	if err != nil {
		return nil, err
	}
	names := []*Alias{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		a, err := p.parseDottedAlias()
		if err != nil {
			return nil, err
		}
		names = append(names, a)
	}
	return &Import{P: pos, Names: names}, nil
}

func (p *parser) parseFromImportStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	level := 0
	for p.cur.kind == tkDot || p.cur.kind == tkEllipsis {
		if p.cur.kind == tkDot {
			level++
		} else {
			level += 3
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	module := ""
	if p.cur.kind == tkName && !isReservedKeyword(p.cur.val) {
		mod, err := p.parseDottedName()
		if err != nil {
			return nil, err
		}
		module = mod
	}
	if !p.isKeyword("import") {
		return nil, fmt.Errorf("%d:%d: expected 'import' in from-import",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.cur.kind == tkStar {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &ImportFrom{P: pos, Module: module, Names: []*Alias{{P: pos, Name: "*"}}, Level: level}, nil
	}
	parens := false
	if p.cur.kind == tkLParen {
		parens = true
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	first, err := p.parseImportAsName()
	if err != nil {
		return nil, err
	}
	names := []*Alias{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if parens && p.cur.kind == tkRParen {
			break
		}
		a, err := p.parseImportAsName()
		if err != nil {
			return nil, err
		}
		names = append(names, a)
	}
	if parens {
		if _, err := p.expect(tkRParen); err != nil {
			return nil, err
		}
	}
	return &ImportFrom{P: pos, Module: module, Names: names, Level: level}, nil
}

func (p *parser) parseDottedAlias() (*Alias, error) {
	pos := p.cur.pos
	name, err := p.parseDottedName()
	if err != nil {
		return nil, err
	}
	asname := ""
	if p.isKeyword("as") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected name after 'as'",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		asname = p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return &Alias{P: pos, Name: name, Asname: asname}, nil
}

func (p *parser) parseImportAsName() (*Alias, error) {
	pos := p.cur.pos
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected name in import list",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	asname := ""
	if p.isKeyword("as") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected name after 'as'",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		asname = p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return &Alias{P: pos, Name: name, Asname: asname}, nil
}

func (p *parser) parseDottedName() (string, error) {
	if p.cur.kind != tkName {
		return "", fmt.Errorf("%d:%d: expected name, got %s",
			p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
	}
	parts := []string{p.cur.val}
	if err := p.advance(); err != nil {
		return "", err
	}
	for p.cur.kind == tkDot {
		if err := p.advance(); err != nil {
			return "", err
		}
		if p.cur.kind != tkName {
			return "", fmt.Errorf("%d:%d: expected name after '.'",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		parts = append(parts, p.cur.val)
		if err := p.advance(); err != nil {
			return "", err
		}
	}
	return strings.Join(parts, "."), nil
}

func (p *parser) parseGlobalNonlocal(global bool) (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected name", p.cur.pos.Line, p.cur.pos.Col)
	}
	names := []string{p.cur.val}
	if err := p.advance(); err != nil {
		return nil, err
	}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected name", p.cur.pos.Line, p.cur.pos.Col)
		}
		names = append(names, p.cur.val)
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if global {
		return &Global{P: pos, Names: names}, nil
	}
	return &Nonlocal{P: pos, Names: names}, nil
}

func (p *parser) parseDeleteStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	targets := []Expr{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		// Trailing comma is allowed: `del a, b,` ends here.
		if isStmtEnd(p.cur.kind) {
			break
		}
		next, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		targets = append(targets, next)
	}
	for _, t := range targets {
		if err := validateAssignTarget(t); err != nil {
			return nil, err
		}
	}
	return &Delete{P: pos, Targets: targets}, nil
}

func (p *parser) parseAssertStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var msg Expr
	if p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		msg, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	return &Assert{P: pos, Test: test, Msg: msg}, nil
}

// ----- Compound statements -----

func isCompoundKeyword(t token) bool {
	if t.kind != tkName {
		return false
	}
	switch t.val {
	case "if", "while", "for", "try", "with", "def", "class", "async":
		return true
	}
	if t.val == "match" {
		// match is a soft keyword. Treat it as compound only if it
		// looks like the start of a match statement; lookahead would be
		// needed to be precise. For v0.1.30 we don't ship match.
		return false
	}
	// Decorators look like ordinary statements but they always come
	// before def / async def / class.
	return false
}

func (p *parser) parseCompoundStmt() (Stmt, error) {
	if p.cur.kind == tkAt {
		return p.parseDecorated()
	}
	if !p.isKeyword("if") && !p.isKeyword("while") && !p.isKeyword("for") &&
		!p.isKeyword("try") && !p.isKeyword("with") && !p.isKeyword("def") &&
		!p.isKeyword("class") && !p.isKeyword("async") {
		return nil, fmt.Errorf("%d:%d: unexpected compound keyword %q",
			p.cur.pos.Line, p.cur.pos.Col, p.cur.val)
	}
	switch p.cur.val {
	case "if":
		return p.parseIfStmt()
	case "while":
		return p.parseWhileStmt()
	case "for":
		return p.parseForStmt(false)
	case "try":
		return p.parseTryStmt()
	case "with":
		return p.parseWithStmt(false)
	case "def":
		return p.parseFunctionDef(nil, false)
	case "class":
		return p.parseClassDef(nil)
	case "async":
		return p.parseAsyncStmt()
	}
	return nil, fmt.Errorf("%d:%d: unreachable compound dispatch",
		p.cur.pos.Line, p.cur.pos.Col)
}

func (p *parser) parseDecorated() (Stmt, error) {
	var decorators []Expr
	for p.cur.kind == tkAt {
		if err := p.advance(); err != nil {
			return nil, err
		}
		dec, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		decorators = append(decorators, dec)
		if p.cur.kind != tkNewline {
			return nil, fmt.Errorf("%d:%d: expected newline after decorator",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if p.isKeyword("def") {
		return p.parseFunctionDef(decorators, false)
	}
	if p.isKeyword("async") {
		// `async def` after decorators.
		if err := p.advance(); err != nil {
			return nil, err
		}
		if !p.isKeyword("def") {
			return nil, fmt.Errorf("%d:%d: 'async' must be followed by 'def' after decorators",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		return p.parseFunctionDef(decorators, true)
	}
	if p.isKeyword("class") {
		return p.parseClassDef(decorators)
	}
	return nil, fmt.Errorf("%d:%d: decorators must precede def/class",
		p.cur.pos.Line, p.cur.pos.Col)
}

func (p *parser) parseAsyncStmt() (Stmt, error) {
	if err := p.advance(); err != nil {
		return nil, err
	}
	switch {
	case p.isKeyword("def"):
		return p.parseFunctionDef(nil, true)
	case p.isKeyword("for"):
		return p.parseForStmt(true)
	case p.isKeyword("with"):
		return p.parseWithStmt(true)
	}
	return nil, fmt.Errorf("%d:%d: 'async' must be followed by 'def', 'for', or 'with'",
		p.cur.pos.Line, p.cur.pos.Col)
}

func (p *parser) parseIfStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	var orelse []Stmt
	switch {
	case p.isKeyword("elif"):
		// Lower elif into a nested If inside orelse so the tree stays
		// uniform. The recursive call will eat further elif/else.
		nested, err := p.parseIfStmt()
		if err != nil {
			return nil, err
		}
		orelse = []Stmt{nested}
	case p.isKeyword("else"):
		if err := p.advance(); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}
	return &If{P: pos, Test: test, Body: body, Orelse: orelse}, nil
}

func (p *parser) parseWhileStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	test, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	var orelse []Stmt
	if p.isKeyword("else") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}
	return &While{P: pos, Test: test, Body: body, Orelse: orelse}, nil
}

func (p *parser) parseForStmt(async bool) (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	target, err := p.parseTargetList()
	if err != nil {
		return nil, err
	}
	if !p.isKeyword("in") {
		return nil, fmt.Errorf("%d:%d: expected 'in' in for-statement",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	iter, err := p.parseTestlistOrStarExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	var orelse []Stmt
	if p.isKeyword("else") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}
	if async {
		return &AsyncFor{P: pos, Target: target, Iter: iter, Body: body, Orelse: orelse}, nil
	}
	return &For{P: pos, Target: target, Iter: iter, Body: body, Orelse: orelse}, nil
}

func (p *parser) parseTryStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	var handlers []*ExceptHandler
	for p.isKeyword("except") {
		hpos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		// PEP 654 `except*` is treated like `except` for v0.1.30; the
		// extra grouping semantics belong to the runtime, not the
		// parse.
		if p.cur.kind == tkStar {
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		var typ Expr
		var name string
		if p.cur.kind != tkColon {
			typ, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
			if p.isKeyword("as") {
				if err := p.advance(); err != nil {
					return nil, err
				}
				if p.cur.kind != tkName {
					return nil, fmt.Errorf("%d:%d: expected name after 'as'",
						p.cur.pos.Line, p.cur.pos.Col)
				}
				name = p.cur.val
				if err := p.advance(); err != nil {
					return nil, err
				}
			}
		}
		hbody, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, &ExceptHandler{P: hpos, Type: typ, Name: name, Body: hbody})
	}
	var orelse []Stmt
	if p.isKeyword("else") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		orelse, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}
	var finalbody []Stmt
	if p.isKeyword("finally") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		finalbody, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	}
	if len(handlers) == 0 && len(finalbody) == 0 {
		return nil, fmt.Errorf("%d:%d: try statement requires except or finally",
			pos.Line, pos.Col)
	}
	return &Try{P: pos, Body: body, Handlers: handlers, Orelse: orelse, Finalbody: finalbody}, nil
}

func (p *parser) parseWithStmt(async bool) (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	parens := false
	if p.cur.kind == tkLParen {
		// PEP 617 parenthesised with-items. Be lenient: if the parens
		// turn out to wrap a single tuple expression rather than items
		// we'll fall back. For v0.1.30 we accept the common case where
		// all items are listed inside one paren.
		parens = true
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	first, err := p.parseWithItem()
	if err != nil {
		return nil, err
	}
	items := []*WithItem{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if parens && p.cur.kind == tkRParen {
			break
		}
		it, err := p.parseWithItem()
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if parens {
		if _, err := p.expect(tkRParen); err != nil {
			return nil, err
		}
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if async {
		return &AsyncWith{P: pos, Items: items, Body: body}, nil
	}
	return &With{P: pos, Items: items, Body: body}, nil
}

func (p *parser) parseWithItem() (*WithItem, error) {
	pos := p.cur.pos
	ctx, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var vars Expr
	if p.isKeyword("as") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		// Single target only: a name, tuple-in-parens, or list. Using
		// parseTargetList here would eat the top-level commas that
		// separate with-items.
		vars, err = p.parseTargetAtom()
		if err != nil {
			return nil, err
		}
	}
	return &WithItem{P: pos, ContextExpr: ctx, OptionalVars: vars}, nil
}

func (p *parser) parseFunctionDef(decorators []Expr, async bool) (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume 'def'
		return nil, err
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected function name",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tkLParen); err != nil {
		return nil, err
	}
	args, err := p.parseFuncParams(tkRParen)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tkRParen); err != nil {
		return nil, err
	}
	var returns Expr
	if p.cur.kind == tkArrow {
		if err := p.advance(); err != nil {
			return nil, err
		}
		returns, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if async {
		return &AsyncFunctionDef{P: pos, Name: name, Args: args, Body: body, DecoratorList: decorators, Returns: returns}, nil
	}
	return &FunctionDef{P: pos, Name: name, Args: args, Body: body, DecoratorList: decorators, Returns: returns}, nil
}

func (p *parser) parseClassDef(decorators []Expr) (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume 'class'
		return nil, err
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected class name",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	var bases []Expr
	var keywords []*Keyword
	if p.cur.kind == tkLParen {
		var err error
		bases, keywords, _, err = p.parseCallArgs()
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ClassDef{P: pos, Name: name, Bases: bases, Keywords: keywords, Body: body, DecoratorList: decorators}, nil
}

// parseBlock reads `: NEWLINE INDENT stmts DEDENT` or, in the inline
// form, `: simple_stmt_list` on the same line.
func (p *parser) parseBlock() ([]Stmt, error) {
	if _, err := p.expect(tkColon); err != nil {
		return nil, err
	}
	if p.cur.kind == tkNewline {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkIndent {
			return nil, fmt.Errorf("%d:%d: expected indented block",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		var stmts []Stmt
		for p.cur.kind != tkDedent && p.cur.kind != tkEOF {
			if p.cur.kind == tkNewline {
				if err := p.advance(); err != nil {
					return nil, err
				}
				continue
			}
			ss, err := p.parseStatementLine()
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, ss...)
		}
		if p.cur.kind == tkDedent {
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		return stmts, nil
	}
	// Inline block: one or more simple statements on the same line.
	return p.parseSimpleStmtList()
}

func isStmtEnd(k tokKind) bool {
	switch k {
	case tkSemi, tkNewline, tkEOF, tkDedent:
		return true
	}
	return false
}

// parseFuncParams parses a function parameter list up to the closing
// token. It's similar in shape to parseLambdaArgs but supports type
// annotations and the `/` and `*` separators.
func (p *parser) parseFuncParams(end tokKind) (*Arguments, error) {
	args := &Arguments{}
	if p.cur.kind == end {
		return args, nil
	}
	state := paramStateNormal
	for {
		switch p.cur.kind {
		case tkSlash:
			if state != paramStateNormal {
				return nil, fmt.Errorf("%d:%d: '/' must follow positional parameters",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			args.PosOnly = append(args.PosOnly, args.Args...)
			args.Args = nil
			if err := p.advance(); err != nil {
				return nil, err
			}
		case tkStar:
			if state == paramStateKwOnly {
				return nil, fmt.Errorf("%d:%d: duplicate '*'",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind == tkName {
				a, err := p.parseFuncParam()
				if err != nil {
					return nil, err
				}
				args.Vararg = a
			}
			state = paramStateKwOnly
		case tkDoubleStar:
			if err := p.advance(); err != nil {
				return nil, err
			}
			a, err := p.parseFuncParam()
			if err != nil {
				return nil, err
			}
			args.Kwarg = a
			state = paramStateDone
		case tkName:
			a, err := p.parseFuncParam()
			if err != nil {
				return nil, err
			}
			var defValue Expr
			if p.cur.kind == tkAssign {
				if err := p.advance(); err != nil {
					return nil, err
				}
				defValue, err = p.parseExpr()
				if err != nil {
					return nil, err
				}
			}
			if state == paramStateKwOnly {
				args.KwOnly = append(args.KwOnly, a)
				args.KwOnlyDef = append(args.KwOnlyDef, defValue)
			} else {
				args.Args = append(args.Args, a)
				if defValue != nil {
					args.Defaults = append(args.Defaults, defValue)
				}
			}
		default:
			return nil, fmt.Errorf("%d:%d: unexpected %s in parameter list",
				p.cur.pos.Line, p.cur.pos.Col, p.cur.kind)
		}
		if p.cur.kind != tkComma {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == end {
			break
		}
	}
	return args, nil
}

func (p *parser) parseFuncParam() (*Arg, error) {
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected parameter name",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	pos := p.cur.pos
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	var ann Expr
	if p.cur.kind == tkColon {
		if err := p.advance(); err != nil {
			return nil, err
		}
		var err error
		ann, err = p.parseExpr()
		if err != nil {
			return nil, err
		}
	}
	return &Arg{P: pos, Name: name, Annotation: ann}, nil
}

const (
	paramStateNormal = iota
	paramStateKwOnly
	paramStateDone
)
