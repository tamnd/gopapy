package parser

import (
	plexer "github.com/alecthomas/participle/v2/lexer"
)

// The grammar tree types. Participle parses into these structs by reading
// the `parser:""` tags. Almost every node carries a Pos field for source
// position; the AST emitter copies the start position into the AST node.
//
// Conventions:
//   - Capture into a field with @ (single token) or @@ (sub-rule).
//   - Right-recursive lists fold into left-leaning binops in fold.go.
//   - Optional alternatives use *Foo plus a non-nil check.
//   - Soft keywords (`match`, `case`, `_`, `type`) come through as NAME.
//   - Hard keywords (`if`, `def`, `for`, ...) are matched by literal value.
//   - Literal-only tokens are inlined into the parser tag of the next
//     captured field — participle skips unexported (`_`) fields entirely
//     when building the grammar, so a tag on `_ struct{}` is silently lost.

// File is the entry point: the whole module body. ENDMARKER is consumed
// optionally because the lexer adapter emits it before EOF.
type File struct {
	Pos        plexer.Position
	Statements []*Statement `parser:"@@* ENDMARKER?"`
	EndPos     plexer.Position
}

// Statement is one top-level statement: simple_stmts or compound_stmt.
type Statement struct {
	Pos      plexer.Position
	Compound *CompoundStmt `parser:"  @@"`
	Simples  *SimpleStmts  `parser:"| @@"`
}

// SimpleStmts is one or more simple statements separated by ';' and
// terminated by NEWLINE.
type SimpleStmts struct {
	Pos   plexer.Position
	First *SimpleStmt   `parser:"@@"`
	Rest  []*SimpleStmt `parser:"( SEMI @@ )* SEMI? NEWLINE"`
}

// SimpleStmt is one of the leaf statements that can appear in a SimpleStmts
// group. Order matters: assignment alternatives come first so an expression
// like `a = 1` parses as Assign, not Expr(`a`) followed by `= 1`.
type SimpleStmt struct {
	Pos plexer.Position

	Return   *ReturnStmt   `parser:"  @@"`
	Pass     bool          `parser:"| @'pass'"`
	Break    bool          `parser:"| @'break'"`
	Continue bool          `parser:"| @'continue'"`
	Raise    *RaiseStmt    `parser:"| @@"`
	Del      *DelStmt      `parser:"| @@"`
	Global   *GlobalStmt   `parser:"| @@"`
	Nonlocal *NonlocalStmt `parser:"| @@"`
	Assert   *AssertStmt   `parser:"| @@"`
	Import   *ImportStmt   `parser:"| @@"`
	From     *FromStmt     `parser:"| @@"`
	Yield    *YieldStmt    `parser:"| @@"`
	Assign   *AssignStmt   `parser:"| @@"`
	ExprStmt *Expression   `parser:"| @@"`
}

// ReturnStmt: `return` followed by an optional expression list. Multiple
// values fold into a Tuple at AST emit time.
type ReturnStmt struct {
	Pos    plexer.Position
	Values []*Expression `parser:"'return' ( @@ ( COMMA @@ )* )?"`
}

type RaiseStmt struct {
	Pos   plexer.Position
	Exc   *Expression `parser:"'raise' ( @@"`
	Cause *Expression `parser:"  ( 'from' @@ )? )?"`
}

type DelStmt struct {
	Pos     plexer.Position
	Targets []*Expression `parser:"'del' @@ ( COMMA @@ )*"`
}

type GlobalStmt struct {
	Pos   plexer.Position
	Names []string `parser:"'global' @NAME ( COMMA @NAME )*"`
}

type NonlocalStmt struct {
	Pos   plexer.Position
	Names []string `parser:"'nonlocal' @NAME ( COMMA @NAME )*"`
}

type AssertStmt struct {
	Pos  plexer.Position
	Test *Expression `parser:"'assert' @@"`
	Msg  *Expression `parser:"( COMMA @@ )?"`
}

type ImportStmt struct {
	Pos   plexer.Position
	Names []*DottedAsName `parser:"'import' @@ ( COMMA @@ )*"`
}

type DottedAsName struct {
	Pos    plexer.Position
	Name   *DottedName `parser:"@@"`
	Asname string      `parser:"( 'as' @NAME )?"`
}

type DottedName struct {
	Pos   plexer.Position
	Parts []string `parser:"@NAME ( DOT @NAME )*"`
}

// FromStmt: relative dots come first; the module is only present if the
// next token isn't `import`. The negative lookahead is the only way to keep
// `import` (which lexes as a NAME) from being slurped into DottedName.
type FromStmt struct {
	Pos    plexer.Position
	Dots   []string    `parser:"'from' @( DOT | ELLIPSIS )*"`
	Module *DottedName `parser:"( (?! 'import') @@ )? 'import'"`
	Star   bool        `parser:"( @STAR"`
	Group  []*ImportAs `parser:"  | LPAREN @@ ( COMMA @@ )* COMMA? RPAREN"`
	Plain  []*ImportAs `parser:"  | @@ ( COMMA @@ )* )"`
}

type ImportAs struct {
	Pos    plexer.Position
	Name   string `parser:"@NAME"`
	Asname string `parser:"( 'as' @NAME )?"`
}

type YieldStmt struct {
	Pos  plexer.Position
	Expr *YieldExpr `parser:"@@"`
}

// AssignStmt covers Assign (one or more `target = ...`), AugAssign, and
// AnnAssign. The disambiguation happens during AST emission based on
// which operator showed up between targets and value.
type AssignStmt struct {
	Pos    plexer.Position
	Target *Expression   `parser:"@@"`
	Annot  *Expression   `parser:"( COLON @@"`
	AnnVal *Expression   `parser:"  ( EQ @@ )? )?"`
	Aug    string        `parser:"  ( @( PLUSEQ | MINUSEQ | STAREQ | SLASHEQ | DOUBLESLEQ | PERCENTEQ | ATEQ | AMPEQ | PIPEEQ | CARETEQ | LSHIFTEQ | RSHIFTEQ | DOUBLESTAREQ )"`
	AugVal *Expression   `parser:"    @@"`
	More   []*Expression `parser:"  | ( EQ @@ )+ )?"`
}

// ---------------------------------------------------------------------------
// Compound statements
// ---------------------------------------------------------------------------

type CompoundStmt struct {
	Pos        plexer.Position
	Decorated  *Decorated `parser:"  @@"`
	If         *IfStmt    `parser:"| @@"`
	While      *WhileStmt `parser:"| @@"`
	For        *ForStmt   `parser:"| @@"`
	With       *WithStmt  `parser:"| @@"`
	Try        *TryStmt   `parser:"| @@"`
	FuncDef    *FuncDef   `parser:"| @@"`
	ClassDef   *ClassDef  `parser:"| @@"`
}

// Decorated is one or more `@expr NEWLINE` lines followed by a function or
// class definition. The expressions can be any callable: a name, a dotted
// path, or a call. CPython's grammar (3.9+) accepts `@expr` rather than the
// older restricted dotted_name(args) form, so we follow suit.
type Decorated struct {
	Pos        plexer.Position
	Decorators []*Decorator `parser:"@@+"`
	FuncDef    *FuncDef     `parser:"( @@"`
	ClassDef   *ClassDef    `parser:"| @@ )"`
}

type Decorator struct {
	Pos  plexer.Position
	Expr *Expression `parser:"AT @@ NEWLINE"`
}

type Block struct {
	Pos  plexer.Position
	Body []*Statement `parser:"NEWLINE INDENT @@+ DEDENT"`
}

type IfStmt struct {
	Pos   plexer.Position
	Test  *Expression   `parser:"'if' @@ COLON"`
	Body  *Block        `parser:"@@"`
	Elifs []*ElifClause `parser:"@@*"`
	Else  *ElseClause   `parser:"@@?"`
}

type ElifClause struct {
	Pos  plexer.Position
	Test *Expression `parser:"'elif' @@ COLON"`
	Body *Block      `parser:"@@"`
}

type ElseClause struct {
	Pos  plexer.Position
	Body *Block `parser:"'else' COLON @@"`
}

type WhileStmt struct {
	Pos  plexer.Position
	Test *Expression `parser:"'while' @@ COLON"`
	Body *Block      `parser:"@@"`
	Else *ElseClause `parser:"@@?"`
}

// ForStmt's target is a star_targets in CPython's grammar — it cannot
// contain a comparison, otherwise the `in` keyword would be consumed by
// Expression's Comparison rule. We use a dedicated TargetList that bottoms
// out at BitOr so `in` stays available as the loop separator.
type ForStmt struct {
	Pos    plexer.Position
	Target *TargetList `parser:"'for' @@ 'in'"`
	Iter   *Expression `parser:"@@ COLON"`
	Body   *Block      `parser:"@@"`
	Else   *ElseClause `parser:"@@?"`
}

// TargetList is one or more comma-separated target atoms. A bare list of
// targets with no trailing comma is a Tuple in the AST; a single target
// stays as the underlying expression.
type TargetList struct {
	Pos       plexer.Position
	Head      *TargetAtom   `parser:"@@"`
	Tail      []*TargetAtom `parser:"( COMMA @@ )*"`
	HasTrail  bool          `parser:"@COMMA?"`
}

// TargetAtom captures a single assignment target. `*x` is allowed for
// starred targets in tuple unpacking.
type TargetAtom struct {
	Pos  plexer.Position
	Star bool   `parser:"@STAR?"`
	Expr *BitOr `parser:"@@"`
}

// WithStmt accepts both the bare comma-separated form
// (`with a, b as c:`) and the parenthesized form added in PEP 617
// (`with (a, b as c, d):`), with an optional trailing comma inside the
// parens. The two forms produce identical AST shapes.
type WithStmt struct {
	Pos        plexer.Position
	Paren      bool        `parser:"'with' ( @LPAREN"`
	Items      []*WithItem `parser:"  @@ ( COMMA @@ )* COMMA? RPAREN"`
	BareItems  []*WithItem `parser:"| @@ ( COMMA @@ )* )"`
	Body       *Block      `parser:"COLON @@"`
}

type WithItem struct {
	Pos     plexer.Position
	Context *Expression `parser:"@@"`
	Vars    *Expression `parser:"( 'as' @@ )?"`
}

type TryStmt struct {
	Pos      plexer.Position
	Body     *Block          `parser:"'try' COLON @@"`
	Handlers []*ExceptClause `parser:"@@*"`
	Else     *ElseClause     `parser:"@@?"`
	Finally  *FinallyClause  `parser:"@@?"`
}

type ExceptClause struct {
	Pos  plexer.Position
	Type *Expression `parser:"'except' ( @@"`
	Name string      `parser:"  ( 'as' @NAME )? )?"`
	Body *Block      `parser:"COLON @@"`
}

type FinallyClause struct {
	Pos  plexer.Position
	Body *Block `parser:"'finally' COLON @@"`
}

type FuncDef struct {
	Pos     plexer.Position
	Name    string      `parser:"'def' @NAME LPAREN"`
	Params  []*Param    `parser:"( @@ ( COMMA @@ )* )? RPAREN"`
	Returns *Expression `parser:"( ARROW @@ )?"`
	Body    *Block      `parser:"COLON @@"`
}

// Param covers one entry in a function parameter list. Five shapes:
//
//   /                  Slash=true                   PEP 570 marker
//   *                  Star=true,  Name=""          bare-star kwonly marker
//   *name              Star=true,  Name=name        vararg
//   **name             Double=true, Name=name       kwarg
//   name[:annot][=default]  the regular case
//
// Annot and Default only ever populate on the regular case. CPython
// rejects them on *name/**name at compile time, not parse, so we accept
// the syntactic form and let downstream flag.
type Param struct {
	Pos     plexer.Position
	Slash   bool        `parser:"  @SLASH"`
	Double  bool        `parser:"| ( @DOUBLESTAR @NAME"`
	Star    bool        `parser:"  | @STAR @NAME? )"`
	Name    string      `parser:"| @NAME"`
	Annot   *Expression `parser:"  ( COLON @@ )?"`
	Default *Expression `parser:"  ( EQ @@ )?"`
}

type ClassDef struct {
	Pos   plexer.Position
	Name  string      `parser:"'class' @NAME"`
	Bases []*Argument `parser:"( LPAREN ( @@ ( COMMA @@ )* )? RPAREN )?"`
	Body  *Block      `parser:"COLON @@"`
}
