package parser

import (
	"fmt"
	"strings"
)

// ParseFile parses src as a complete Python module and returns its
// Module AST. filename is used only for error messages; the bytes
// themselves come from src.
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
	if p.looksLikeMatchStmt() {
		s, err := p.parseMatchStmt()
		if err != nil {
			return nil, err
		}
		return []Stmt{s}, nil
	}
	if p.looksLikeTypeAlias() {
		s, err := p.parseTypeAliasStmt()
		if err != nil {
			return nil, err
		}
		return []Stmt{s}, nil
	}
	return p.parseSimpleStmtList()
}

// looksLikeTypeAlias reports whether the current `type` name is the
// soft keyword introducing a PEP 695 type alias. The trigger is
// `type NAME` at statement start. `type = 1`, `type(x)`, `type.x`,
// `type[K]` etc. remain plain name uses.
func (p *parser) looksLikeTypeAlias() bool {
	if p.cur.kind != tkName || p.cur.val != "type" {
		return false
	}
	nxt, err := p.peekTok()
	if err != nil {
		return false
	}
	if nxt.kind != tkName {
		return false
	}
	return !isReservedKeyword(nxt.val)
}

// looksLikeMatchStmt reports whether the current `match` name token
// is acting as a soft keyword. The next token must be one that can
// start a subject expression in PEP 634; if it's an operator that
// would bind `match` as a name use (`match = ...`, `match(...)`,
// `match.x`, `match[i]`, `match: int`, etc.) we treat `match` as a
// regular name instead.
func (p *parser) looksLikeMatchStmt() bool {
	if p.cur.kind != tkName || p.cur.val != "match" {
		return false
	}
	nxt, err := p.peekTok()
	if err != nil {
		return false
	}
	switch nxt.kind {
	case tkInt, tkFloat, tkString, tkFString, tkLBrace,
		tkMinus, tkPlus, tkTilde, tkStar, tkEllipsis:
		return true
	case tkLParen:
		// `match (expr):` — parenthesised subject. Need to distinguish
		// from `match(expr)` (a function call with no colon after).
		// Do a cheap byte scan: skip past the matching ')' and check
		// whether ':' follows (possibly preceded by spaces).
		return p.sc.parenFollowedByColon()
	case tkLBrack:
		// `match[i]:` is a valid match subject (subscript on the name
		// `match`). But `match[i] = x` is a subscript assignment, not
		// a match statement. Scan past the closing ']' and require ':'.
		return p.sc.parenFollowedByColon()
	case tkName:
		// `match NAME ...` - any name (including `case`, which would
		// be the next case keyword if the body were empty, but the
		// grammar requires at least one case so we still commit).
		return !isReservedKeyword(nxt.val) || nxt.val == "not" ||
			nxt.val == "lambda" || nxt.val == "await" || nxt.val == "yield" ||
			nxt.val == "None" || nxt.val == "True" || nxt.val == "False" ||
			isSoftKeyword(nxt.val)
	}
	return false
}

// parseSimpleStmtList reads one or more semicolon-separated small
// statements terminated by NEWLINE (or EOF).
func (p *parser) parseSimpleStmtList() ([]Stmt, error) {
	var stmts []Stmt
	for {
		var s Stmt
		var err error
		if p.looksLikeTypeAlias() {
			s, err = p.parseTypeAliasStmt()
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, s)
			// parseTypeAliasStmt already consumed the trailing ; and \n.
			return stmts, nil
		}
		s, err = p.parseSimpleStmt()
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
	if p.cur.kind == tkName && (!isReservedKeyword(p.cur.val) || isSoftKeyword(p.cur.val)) {
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
	// `match` is a soft keyword. parseStatementLine handles the
	// soft-keyword lookahead via looksLikeMatchStmt rather than
	// classifying it as a compound keyword here.
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
	isTryStar := false
	for p.isKeyword("except") {
		hpos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		// PEP 654 `except*` — track for TryStar node production.
		if p.cur.kind == tkStar {
			isTryStar = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		var typ Expr
		var name string
		if p.cur.kind != tkColon {
			first, err2 := p.parseExpr()
			if err2 != nil {
				return nil, err2
			}
			typ = first
			// PEP 758: paren-less except-tuple `except A, B:`.
			if p.cur.kind == tkComma {
				tupPos := first.pos()
				elts := []Expr{first}
				for p.cur.kind == tkComma {
					if err := p.advance(); err != nil {
						return nil, err
					}
					if p.cur.kind == tkColon || p.isKeyword("as") {
						break
					}
					next, err2 := p.parseExpr()
					if err2 != nil {
						return nil, err2
					}
					elts = append(elts, next)
				}
				typ = &Tuple{P: tupPos, Elts: elts}
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
	if isTryStar {
		return &TryStar{P: pos, Body: body, Handlers: handlers, Orelse: orelse, Finalbody: finalbody}, nil
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
		// PEP 617 parenthesised with-items. We advance past '(' and
		// parse normally. After consuming the matching ')', we check
		// whether postfix operators follow — in that case the '(' was
		// used for line-continuation around a single expression (e.g.
		// `with (p / 'a').open() as f:` or `with (a if c else b) as f:`),
		// not as a PEP 617 item list.
		parens = true
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	first, err := p.parseWithItem()
	if err != nil {
		return nil, err
	}
	// Inside a parenthesized with, `for` after the first expr means it's a
	// generator expression context manager, not a PEP 617 item list.
	if parens && p.isCompForStart() {
		gens, err := p.parseComprehensionClauses()
		if err != nil {
			return nil, err
		}
		genExpr := &GeneratorExp{P: pos, Elt: first.ContextExpr, Gens: gens}
		first = &WithItem{ContextExpr: genExpr}
	}
	items := []*WithItem{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if parens && p.cur.kind == tkRParen {
			break
		}
		// Inside a parenthesized with, a starred element (*x) at the item
		// level means (…) is a tuple literal, not a PEP 617 with-item list.
		// Collect remaining starred/plain elements into a Tuple.
		if parens && p.cur.kind == tkStar {
			tupleElts := []Expr{first.ContextExpr}
			for {
				elem, err := p.parseStarredOrExpr()
				if err != nil {
					return nil, err
				}
				tupleElts = append(tupleElts, elem)
				if p.cur.kind != tkComma {
					break
				}
				if err := p.advance(); err != nil {
					return nil, err
				}
				if p.cur.kind == tkRParen {
					break
				}
			}
			items = []*WithItem{{ContextExpr: &Tuple{P: pos, Elts: tupleElts}}}
			break
		}
		// Inside a parenthesized with, a walrus (named expression) as the first
		// item's context — followed by a comma — means (…) is a tuple context
		// manager, not a PEP 617 with-item list (same rule CPython uses).
		if parens && first.OptionalVars == nil {
			if _, ok := first.ContextExpr.(*NamedExpr); ok {
				tupleElts := []Expr{first.ContextExpr}
				for {
					elem, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					tupleElts = append(tupleElts, elem)
					if p.cur.kind != tkComma {
						break
					}
					if err := p.advance(); err != nil {
						return nil, err
					}
					if p.cur.kind == tkRParen {
						break
					}
				}
				items = []*WithItem{{ContextExpr: &Tuple{P: pos, Elts: tupleElts}}}
				break
			}
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
		// If only one item has no 'as' target yet and a postfix operator
		// follows, the '(…)' was a grouping paren around a single context
		// manager expression. Continue parsing the trailer (attribute
		// access, subscript, call) onto that expression, then check for
		// an 'as' target.
		if len(items) == 1 && items[0].OptionalVars == nil {
			ctx := items[0].ContextExpr
			for p.cur.kind == tkDot || p.cur.kind == tkLBrack || p.cur.kind == tkLParen {
				switch p.cur.kind {
				case tkDot:
					dpos := p.cur.pos
					if err := p.advance(); err != nil {
						return nil, err
					}
					if p.cur.kind != tkName {
						return nil, fmt.Errorf("%d:%d: expected name after '.'",
							p.cur.pos.Line, p.cur.pos.Col)
					}
					attr := p.cur.val
					if err := p.advance(); err != nil {
						return nil, err
					}
					ctx = &Attribute{P: dpos, Value: ctx, Attr: attr}
				case tkLParen:
					args, kwargs, callPos, err := p.parseCallArgs()
					if err != nil {
						return nil, err
					}
					ctx = &Call{P: callPos, Func: ctx, Args: args, Keywords: kwargs}
				case tkLBrack:
					spos := p.cur.pos
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
					ctx = &Subscript{P: spos, Value: ctx, Slice: slice}
				}
			}
			items[0].ContextExpr = ctx
			// Re-check for an 'as' target after the postfix tail.
			if p.isKeyword("as") {
				if err := p.advance(); err != nil {
					return nil, err
				}
				vars, err := p.parseTrailer()
				if err != nil {
					return nil, err
				}
				items[0].OptionalVars = vars
			}
			// If a comma follows, (expr) was a parenthesized single item in
			// a multi-item with-statement: `with (a), (b):`. Parse the rest.
			for p.cur.kind == tkComma {
				if err := p.advance(); err != nil {
					return nil, err
				}
				if isStmtEnd(p.cur.kind) || p.cur.kind == tkColon {
					break
				}
				it, err := p.parseWithItem()
				if err != nil {
					return nil, err
				}
				items = append(items, it)
			}
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
		// Parse the as-target using parseTrailer so that call+subscript
		// chains are accepted: `with f() as g()[0][1]:` is valid Python.
		// We do NOT use parseTargetList because the top-level commas
		// must stay available for separating with-items.
		vars, err = p.parseTrailer()
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
	var typeParams []TypeParam
	if p.cur.kind == tkLBrack {
		var err error
		typeParams, err = p.parseTypeParams()
		if err != nil {
			return nil, err
		}
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
		return &AsyncFunctionDef{P: pos, Name: name, TypeParams: typeParams, Args: args, Body: body, DecoratorList: decorators, Returns: returns}, nil
	}
	return &FunctionDef{P: pos, Name: name, TypeParams: typeParams, Args: args, Body: body, DecoratorList: decorators, Returns: returns}, nil
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
	var typeParams []TypeParam
	if p.cur.kind == tkLBrack {
		var err error
		typeParams, err = p.parseTypeParams()
		if err != nil {
			return nil, err
		}
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
	return &ClassDef{P: pos, Name: name, TypeParams: typeParams, Bases: bases, Keywords: keywords, Body: body, DecoratorList: decorators}, nil
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
		// PEP 646: `*args: *Ts` — starred annotation expression.
		annPos := p.cur.pos
		var err error
		if p.cur.kind == tkStar {
			if err = p.advance(); err != nil {
				return nil, err
			}
			var inner Expr
			inner, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
			ann = &Starred{P: annPos, Value: inner}
		} else {
			ann, err = p.parseExpr()
		}
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

// ----- Match statement (PEP 634) -----

func (p *parser) parseMatchStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil {
		return nil, err
	}
	subject, err := p.parseTestlistOrStarExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tkColon); err != nil {
		return nil, err
	}
	if p.cur.kind != tkNewline {
		return nil, fmt.Errorf("%d:%d: expected newline after match header",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.cur.kind != tkIndent {
		return nil, fmt.Errorf("%d:%d: expected indented block in match",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	var cases []*MatchCase
	for p.cur.kind != tkDedent && p.cur.kind != tkEOF {
		if p.cur.kind == tkNewline {
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		if p.cur.kind != tkName || p.cur.val != "case" {
			return nil, fmt.Errorf("%d:%d: expected 'case' in match block",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		c, err := p.parseCaseArm()
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	if p.cur.kind == tkDedent {
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("%d:%d: match statement requires at least one case",
			pos.Line, pos.Col)
	}
	return &Match{P: pos, Subject: subject, Cases: cases}, nil
}

func (p *parser) parseCaseArm() (*MatchCase, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume `case`
		return nil, err
	}
	pat, err := p.parsePatterns()
	if err != nil {
		return nil, err
	}
	var guard Expr
	if p.cur.kind == tkName && p.cur.val == "if" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		g, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		guard = g
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &MatchCase{P: pos, Pattern: pat, Guard: guard, Body: body}, nil
}

// parsePatterns is the case-arm entry. It allows a paren-less open
// sequence (`case 0, *rest:`), but otherwise delegates to a single
// as/or pattern.
func (p *parser) parsePatterns() (Pattern, error) {
	pos := p.cur.pos
	first, isStar, err := p.parseMaybeStarPattern()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkComma {
		if isStar {
			return nil, fmt.Errorf("%d:%d: bare star pattern at top level",
				pos.Line, pos.Col)
		}
		return first, nil
	}
	items := []Pattern{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkColon || (p.cur.kind == tkName && p.cur.val == "if") {
			break
		}
		next, _, err := p.parseMaybeStarPattern()
		if err != nil {
			return nil, err
		}
		items = append(items, next)
	}
	return &MatchSequence{P: pos, Patterns: items}, nil
}

// parseMaybeStarPattern parses either a `*name`/`*_` star pattern or
// a normal as/or pattern. The bool tells the caller whether the
// returned pattern was the star form.
func (p *parser) parseMaybeStarPattern() (Pattern, bool, error) {
	if p.cur.kind == tkStar {
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		if p.cur.kind != tkName {
			return nil, false, fmt.Errorf("%d:%d: expected name after '*' in pattern",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		name := p.cur.val
		if err := p.advance(); err != nil {
			return nil, false, err
		}
		if name == "_" {
			return &MatchStar{P: pos}, true, nil
		}
		return &MatchStar{P: pos, Name: name}, true, nil
	}
	pat, err := p.parseAsPattern()
	if err != nil {
		return nil, false, err
	}
	return pat, false, nil
}

func (p *parser) parseAsPattern() (Pattern, error) {
	pos := p.cur.pos
	or, err := p.parseOrPattern()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkName || p.cur.val != "as" {
		return or, nil
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.cur.kind != tkName || (isReservedKeyword(p.cur.val) && !isSoftKeyword(p.cur.val)) {
		return nil, fmt.Errorf("%d:%d: expected capture name after 'as'",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	name := p.cur.val
	if name == "_" {
		return nil, fmt.Errorf("%d:%d: cannot use '_' as 'as' target",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return &MatchAs{P: pos, Pattern: or, Name: name}, nil
}

func (p *parser) parseOrPattern() (Pattern, error) {
	pos := p.cur.pos
	first, err := p.parseClosedPattern()
	if err != nil {
		return nil, err
	}
	if p.cur.kind != tkPipe {
		return first, nil
	}
	items := []Pattern{first}
	for p.cur.kind == tkPipe {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseClosedPattern()
		if err != nil {
			return nil, err
		}
		items = append(items, next)
	}
	return &MatchOr{P: pos, Patterns: items}, nil
}

func (p *parser) parseClosedPattern() (Pattern, error) {
	pos := p.cur.pos
	switch p.cur.kind {
	case tkInt, tkFloat, tkString, tkFString, tkMinus, tkPlus:
		return p.parseLiteralPattern()
	case tkLBrack:
		return p.parseSequencePatternBrackets()
	case tkLParen:
		return p.parseGroupOrSequencePattern()
	case tkLBrace:
		return p.parseMappingPattern()
	case tkName:
		switch p.cur.val {
		case "None":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &MatchSingleton{P: pos, Value: nil}, nil
		case "True":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &MatchSingleton{P: pos, Value: true}, nil
		case "False":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &MatchSingleton{P: pos, Value: false}, nil
		case "_":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &MatchAs{P: pos}, nil
		}
		return p.parseNameStartedPattern()
	}
	return nil, fmt.Errorf("%d:%d: unexpected token %s in pattern",
		pos.Line, pos.Col, p.cur.kind)
}

// parseLiteralPattern handles signed numbers (incl. `a + bj`) and
// strings. The value is wrapped in a MatchValue node.
func (p *parser) parseLiteralPattern() (Pattern, error) {
	pos := p.cur.pos
	if p.cur.kind == tkString || p.cur.kind == tkFString {
		s, err := p.parseStringAtom()
		if err != nil {
			return nil, err
		}
		return &MatchValue{P: pos, Value: s}, nil
	}
	real, err := p.parseSignedNumberLiteral()
	if err != nil {
		return nil, err
	}
	if p.cur.kind == tkPlus || p.cur.kind == tkMinus {
		op := "Add"
		if p.cur.kind == tkMinus {
			op = "Sub"
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkInt && p.cur.kind != tkFloat {
			return nil, fmt.Errorf("%d:%d: expected imaginary literal after sign",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		imag, err := p.parseNumberLiteral()
		if err != nil {
			return nil, err
		}
		return &MatchValue{P: pos, Value: &BinOp{P: pos, Op: op, Left: real, Right: imag}}, nil
	}
	return &MatchValue{P: pos, Value: real}, nil
}

func (p *parser) parseSignedNumberLiteral() (Expr, error) {
	if p.cur.kind == tkPlus || p.cur.kind == tkMinus {
		neg := p.cur.kind == tkMinus
		pos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkInt && p.cur.kind != tkFloat {
			return nil, fmt.Errorf("%d:%d: expected number after sign",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		num, err := p.parseNumberLiteral()
		if err != nil {
			return nil, err
		}
		if !neg {
			return &UnaryOp{P: pos, Op: "UAdd", Operand: num}, nil
		}
		return &UnaryOp{P: pos, Op: "USub", Operand: num}, nil
	}
	return p.parseNumberLiteral()
}

func (p *parser) parseNumberLiteral() (Expr, error) {
	tok := p.cur
	if err := p.advance(); err != nil {
		return nil, err
	}
	switch tok.kind {
	case tkInt:
		return parseIntLiteral(tok)
	case tkFloat:
		return parseFloatLiteral(tok)
	}
	return nil, fmt.Errorf("%d:%d: expected number literal",
		tok.pos.Line, tok.pos.Col)
}

// parseNameStartedPattern handles capture, value (dotted attribute),
// and class patterns. Caller has verified cur is a non-special name.
func (p *parser) parseNameStartedPattern() (Pattern, error) {
	pos := p.cur.pos
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.cur.kind != tkDot {
		// Bare name. Class form when followed by `(`, otherwise a
		// capture pattern.
		if p.cur.kind == tkLParen {
			cls := Expr(&Name{P: pos, Id: name})
			return p.parseClassPatternArgs(pos, cls)
		}
		if isReservedKeyword(name) && name != "match" && name != "case" {
			return nil, fmt.Errorf("%d:%d: cannot use reserved name %q as capture",
				pos.Line, pos.Col, name)
		}
		return &MatchAs{P: pos, Name: name}, nil
	}
	// Dotted: build Attribute chain, then either class or value pattern.
	value := Expr(&Name{P: pos, Id: name})
	for p.cur.kind == tkDot {
		dotPos := p.cur.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected attribute name after '.'",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		attr := p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
		value = &Attribute{P: dotPos, Value: value, Attr: attr}
	}
	if p.cur.kind == tkLParen {
		return p.parseClassPatternArgs(pos, value)
	}
	return &MatchValue{P: pos, Value: value}, nil
}

// parseClassPatternArgs parses `(positional, kw=pattern)` after the
// class expression has been consumed. The `(` is the current token.
func (p *parser) parseClassPatternArgs(pos Pos, cls Expr) (Pattern, error) {
	if _, err := p.expect(tkLParen); err != nil {
		return nil, err
	}
	var positional []Pattern
	var kwAttrs []string
	var kwPatterns []Pattern
	seenKw := false
	for p.cur.kind != tkRParen {
		// keyword form: NAME `=` pattern (soft keywords like match/case allowed)
		if p.cur.kind == tkName && (!isReservedKeyword(p.cur.val) || isSoftKeyword(p.cur.val)) {
			nxt, err := p.peekTok()
			if err != nil {
				return nil, err
			}
			if nxt.kind == tkAssign {
				name := p.cur.val
				if err := p.advance(); err != nil {
					return nil, err
				}
				if err := p.advance(); err != nil { // consume =
					return nil, err
				}
				val, err := p.parseAsPattern()
				if err != nil {
					return nil, err
				}
				kwAttrs = append(kwAttrs, name)
				kwPatterns = append(kwPatterns, val)
				seenKw = true
				goto trail
			}
		}
		if seenKw {
			return nil, fmt.Errorf("%d:%d: positional pattern follows keyword pattern",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		{
			pat, err := p.parseAsPattern()
			if err != nil {
				return nil, err
			}
			positional = append(positional, pat)
		}
	trail:
		if p.cur.kind == tkComma {
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		break
	}
	if _, err := p.expect(tkRParen); err != nil {
		return nil, err
	}
	return &MatchClass{
		P:           pos,
		Cls:         cls,
		Patterns:    positional,
		KwdAttrs:    kwAttrs,
		KwdPatterns: kwPatterns,
	}, nil
}

func (p *parser) parseSequencePatternBrackets() (Pattern, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume [
		return nil, err
	}
	var items []Pattern
	for p.cur.kind != tkRBrack {
		item, _, err := p.parseMaybeStarPattern()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.cur.kind != tkComma {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(tkRBrack); err != nil {
		return nil, err
	}
	return &MatchSequence{P: pos, Patterns: items}, nil
}

// parseGroupOrSequencePattern handles `(p)` (group) vs `(p, q)`
// (tuple-form sequence) vs `()` (empty sequence).
func (p *parser) parseGroupOrSequencePattern() (Pattern, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume (
		return nil, err
	}
	if p.cur.kind == tkRParen {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return &MatchSequence{P: pos}, nil
	}
	first, isStar, err := p.parseMaybeStarPattern()
	if err != nil {
		return nil, err
	}
	if p.cur.kind == tkRParen {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if isStar {
			// `(*x)` is an unusual single-star form — treat as 1-tuple.
			return &MatchSequence{P: pos, Patterns: []Pattern{first}}, nil
		}
		return first, nil
	}
	items := []Pattern{first}
	for p.cur.kind == tkComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind == tkRParen {
			break
		}
		next, _, err := p.parseMaybeStarPattern()
		if err != nil {
			return nil, err
		}
		items = append(items, next)
	}
	if _, err := p.expect(tkRParen); err != nil {
		return nil, err
	}
	return &MatchSequence{P: pos, Patterns: items}, nil
}

// parseMappingPattern handles `{ key: pattern, **rest }`.
func (p *parser) parseMappingPattern() (Pattern, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume {
		return nil, err
	}
	var keys []Expr
	var pats []Pattern
	rest := ""
	for p.cur.kind != tkRBrace {
		if p.cur.kind == tkDoubleStar {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind != tkName || isReservedKeyword(p.cur.val) {
				return nil, fmt.Errorf("%d:%d: expected name after '**' in mapping pattern",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			rest = p.cur.val
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind == tkComma {
				if err := p.advance(); err != nil {
					return nil, err
				}
			}
			break
		}
		key, err := p.parseMappingKey()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tkColon); err != nil {
			return nil, err
		}
		val, err := p.parseAsPattern()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		pats = append(pats, val)
		if p.cur.kind != tkComma {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(tkRBrace); err != nil {
		return nil, err
	}
	return &MatchMapping{P: pos, Keys: keys, Patterns: pats, Rest: rest}, nil
}

// parseMappingKey accepts a literal expression (signed number, string,
// complex literal sum, None/True/False) or a dotted attribute access.
func (p *parser) parseMappingKey() (Expr, error) {
	pos := p.cur.pos
	switch p.cur.kind {
	case tkString, tkFString:
		return p.parseStringAtom()
	case tkInt, tkFloat, tkPlus, tkMinus:
		real, err := p.parseSignedNumberLiteral()
		if err != nil {
			return nil, err
		}
		if p.cur.kind == tkPlus || p.cur.kind == tkMinus {
			op := "Add"
			if p.cur.kind == tkMinus {
				op = "Sub"
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			imag, err := p.parseNumberLiteral()
			if err != nil {
				return nil, err
			}
			return &BinOp{P: pos, Op: op, Left: real, Right: imag}, nil
		}
		return real, nil
	case tkName:
		switch p.cur.val {
		case "None":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: pos, Kind: "None"}, nil
		case "True":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: pos, Kind: "True"}, nil
		case "False":
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &Constant{P: pos, Kind: "False"}, nil
		}
		// Dotted name (value-pattern key).
		name := p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
		var v Expr = &Name{P: pos, Id: name}
		for p.cur.kind == tkDot {
			dotPos := p.cur.pos
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.cur.kind != tkName {
				return nil, fmt.Errorf("%d:%d: expected attribute name after '.'",
					p.cur.pos.Line, p.cur.pos.Col)
			}
			attr := p.cur.val
			if err := p.advance(); err != nil {
				return nil, err
			}
			v = &Attribute{P: dotPos, Value: v, Attr: attr}
		}
		return v, nil
	}
	return nil, fmt.Errorf("%d:%d: unexpected token %s in mapping key",
		pos.Line, pos.Col, p.cur.kind)
}

// ----- Type-parameter clause + type alias (PEP 695) -----

func (p *parser) parseTypeAliasStmt() (Stmt, error) {
	pos := p.cur.pos
	if err := p.advance(); err != nil { // consume `type`
		return nil, err
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected name in type alias",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	namePos := p.cur.pos
	nameStr := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	var typeParams []TypeParam
	if p.cur.kind == tkLBrack {
		var err error
		typeParams, err = p.parseTypeParams()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(tkAssign); err != nil {
		return nil, err
	}
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.cur.kind == tkSemi {
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if p.cur.kind == tkNewline {
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return &TypeAlias{
		P:          pos,
		Name:       &Name{P: namePos, Id: nameStr},
		TypeParams: typeParams,
		Value:      value,
	}, nil
}

func (p *parser) parseTypeParams() ([]TypeParam, error) {
	if _, err := p.expect(tkLBrack); err != nil {
		return nil, err
	}
	var params []TypeParam
	for p.cur.kind != tkRBrack {
		tp, err := p.parseTypeParam()
		if err != nil {
			return nil, err
		}
		params = append(params, tp)
		if p.cur.kind != tkComma {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(tkRBrack); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *parser) parseTypeParam() (TypeParam, error) {
	pos := p.cur.pos
	switch p.cur.kind {
	case tkStar:
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected name after '*' in type-parameter list",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		name := p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
		def, err := p.parseTypeParamDefault()
		if err != nil {
			return nil, err
		}
		return &TypeVarTuple{P: pos, Name: name, DefaultValue: def}, nil
	case tkDoubleStar:
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.kind != tkName {
			return nil, fmt.Errorf("%d:%d: expected name after '**' in type-parameter list",
				p.cur.pos.Line, p.cur.pos.Col)
		}
		name := p.cur.val
		if err := p.advance(); err != nil {
			return nil, err
		}
		def, err := p.parseTypeParamDefault()
		if err != nil {
			return nil, err
		}
		return &ParamSpec{P: pos, Name: name, DefaultValue: def}, nil
	}
	if p.cur.kind != tkName {
		return nil, fmt.Errorf("%d:%d: expected type parameter",
			p.cur.pos.Line, p.cur.pos.Col)
	}
	name := p.cur.val
	if err := p.advance(); err != nil {
		return nil, err
	}
	var bound Expr
	if p.cur.kind == tkColon {
		if err := p.advance(); err != nil {
			return nil, err
		}
		b, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		bound = b
	}
	def, err := p.parseTypeParamDefault()
	if err != nil {
		return nil, err
	}
	return &TypeVar{P: pos, Name: name, Bound: bound, DefaultValue: def}, nil
}

func (p *parser) parseTypeParamDefault() (Expr, error) {
	if p.cur.kind != tkAssign {
		return nil, nil
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	// A TypeVarTuple default may be starred: *Ts=*int parses the default
	// as Starred(value=int) per PEP 696 / Python 3.13.
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

