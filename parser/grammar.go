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
	TypeAlias *TypeAliasStmt `parser:"| @@"`
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
	Target *AssignTarget   `parser:"@@"`
	Annot  *Expression     `parser:"( COLON @@"`
	AnnVal *Expression     `parser:"  ( EQ @@ )? )?"`
	Aug    string          `parser:"  ( @( PLUSEQ | MINUSEQ | STAREQ | SLASHEQ | DOUBLESLEQ | PERCENTEQ | ATEQ | AMPEQ | PIPEEQ | CARETEQ | LSHIFTEQ | RSHIFTEQ | DOUBLESTAREQ )"`
	AugVal *Expression     `parser:"    @@"`
	More   []*AssignTarget `parser:"  | ( EQ @@ )+ )?"`
}

// AssignTarget is the comma-separated expression list that appears on either
// side of an `=` in an assignment. Each item may be prefixed with `*` for a
// starred target (`a, *rest = xs`) or for a starred RHS element. With more
// than one item the AST emits a Tuple; a single item emits as the inner
// expression directly.
type AssignTarget struct {
	Pos      plexer.Position
	Head     *StarExpr   `parser:"@@"`
	Tail     []*StarExpr `parser:"( COMMA @@ )*"`
	HasTrail bool        `parser:"@COMMA?"`
}

type StarExpr struct {
	Pos  plexer.Position
	Star bool        `parser:"@STAR?"`
	Expr *Expression `parser:"@@"`
}

// ---------------------------------------------------------------------------
// Compound statements
// ---------------------------------------------------------------------------

type CompoundStmt struct {
	Pos        plexer.Position
	Async      *AsyncStmt `parser:"  @@"`
	Decorated  *Decorated `parser:"| @@"`
	If         *IfStmt    `parser:"| @@"`
	While      *WhileStmt `parser:"| @@"`
	For        *ForStmt   `parser:"| @@"`
	With       *WithStmt  `parser:"| @@"`
	Try        *TryStmt   `parser:"| @@"`
	Match      *MatchStmt `parser:"| @@"`
	FuncDef    *FuncDef   `parser:"| @@"`
	ClassDef   *ClassDef  `parser:"| @@"`
}

// MatchStmt is PEP 634 structural pattern matching. `match` is a soft
// keyword: it lexes as NAME and only acquires statement-keyword status
// when followed by a subject expression and a colon. If neither is
// present, parsing falls through to the SimpleStmt alternatives so
// `match = 1` and `match` (bare expression) still work.
type MatchStmt struct {
	Pos     plexer.Position
	Subject *AssignTarget `parser:"'match' @@ COLON NEWLINE INDENT"`
	Cases   []*CaseClause `parser:"@@+ DEDENT"`
}

type CaseClause struct {
	Pos     plexer.Position
	Pattern *Pattern    `parser:"'case' @@"`
	Guard   *Expression `parser:"( 'if' @@ )?"`
	Body    *Block      `parser:"COLON @@"`
}

// Pattern is the top of the PEP 634 pattern hierarchy: an OrPattern
// followed by an optional `as NAME` capture. The grammar nests as
// Pattern -> OrPattern -> ClosedPattern (with one of eight alternatives).
type Pattern struct {
	Pos plexer.Position
	Or  *OrPattern `parser:"@@"`
	As  string     `parser:"( 'as' @NAME )?"`
}

type OrPattern struct {
	Pos  plexer.Position
	Head *ClosedPattern   `parser:"@@"`
	Tail []*ClosedPattern `parser:"( PIPE @@ )*"`
}

type ClosedPattern struct {
	Pos      plexer.Position
	Class    *ClassPattern `parser:"  @@"`
	Value    *ValuePattern `parser:"| @@"`
	Sequence *SeqPattern   `parser:"| @@"`
	Mapping  *MapPattern   `parser:"| @@"`
	Literal  *LitPattern   `parser:"| @@"`
	Capture  string        `parser:"| @NAME"`
	Group    *Pattern      `parser:"| LPAREN @@ RPAREN"`
}

// ClassPattern is `Name(args)` or `mod.Name(args)`, distinguished from
// CapturePattern / ValuePattern by the trailing parenthesised arg list.
type ClassPattern struct {
	Pos      plexer.Position
	Cls      *PatDotted     `parser:"@@ LPAREN"`
	Args     []*PatternArg  `parser:"( @@ ( COMMA @@ )* COMMA? )? RPAREN"`
}

type PatternArg struct {
	Pos     plexer.Position
	Keyword string   `parser:"( @NAME EQ"`
	Value   *Pattern `parser:"  @@"`
	Pos1    *Pattern `parser:"| @@ )"`
}

// ValuePattern is a dotted name (at least two segments). A single NAME
// is a CapturePattern; only `mod.NAME` and longer count as a value.
type ValuePattern struct {
	Pos  plexer.Position
	Head string   `parser:"@NAME"`
	Tail []string `parser:"( DOT @NAME )+"`
}

type PatDotted struct {
	Pos  plexer.Position
	Head string   `parser:"@NAME"`
	Tail []string `parser:"( DOT @NAME )*"`
}

// SeqPattern is `[ p1, p2, *rest ]` or `( p1, p2 )`. Empty `[]`/`()`
// also produces an empty MatchSequence.
type SeqPattern struct {
	Pos   plexer.Position
	Brack bool          `parser:"( @LBRACK"`
	Items []*SeqItem    `parser:"  ( @@ ( COMMA @@ )* COMMA? )? RBRACK"`
	Paren bool          `parser:"| @LPAREN"`
	PItems []*SeqItem   `parser:"  ( @@ COMMA ( @@ ( COMMA @@ )* COMMA? )? )? RPAREN )"`
}

type SeqItem struct {
	Pos  plexer.Position
	Star *MatchStarItem `parser:"  @@"`
	Pat  *Pattern       `parser:"| @@"`
}

type MatchStarItem struct {
	Pos  plexer.Position
	Name string `parser:"STAR @NAME"`
}

// MapPattern is `{ "k": p1, NAME: p2, **rest }`.
type MapPattern struct {
	Pos   plexer.Position
	Items []*MapItem `parser:"LBRACE ( @@ ( COMMA @@ )* COMMA? )? RBRACE"`
}

type MapItem struct {
	Pos     plexer.Position
	Rest    string   `parser:"  DOUBLESTAR @NAME"`
	Key     *MapKey  `parser:"| @@ COLON"`
	Pattern *Pattern `parser:"  @@"`
}

// MapKey is an expression that resolves to a hashable literal: a literal
// or a value (dotted) pattern. We accept any Expression and let the
// emitter validate.
type MapKey struct {
	Pos    plexer.Position
	Sign   string   `parser:"@( PLUS | MINUS )?"`
	Number string   `parser:"( @NUMBER"`
	String []string `parser:"| @STRING+"`
	True   bool     `parser:"| @'True'"`
	False_ bool     `parser:"| @'False'"`
	None   bool     `parser:"| @'None'"`
	Value  *PatDotted `parser:"| @@ )"`
}

// LitPattern: signed/unsigned numbers, strings, True/False/None.
// Strings are matched as MatchValue(Constant); True/False/None as
// MatchSingleton.
type LitPattern struct {
	Pos    plexer.Position
	Sign   string   `parser:"@( PLUS | MINUS )?"`
	Number string   `parser:"( @NUMBER"`
	Imag   string   `parser:"  ( @( PLUS | MINUS ) @NUMBER )?"`
	String []string `parser:"| @STRING+"`
	True   bool     `parser:"| @'True'"`
	False_ bool     `parser:"| @'False'"`
	None   bool     `parser:"| @'None'"`
	Op     string   `parser:"  )"`
}

// AsyncStmt is the `async` soft-keyword prefix on def, for, or with.
// Each form reuses the existing non-async sub-rule and the AST emitter
// swaps in the AsyncFunctionDef / AsyncFor / AsyncWith node types.
type AsyncStmt struct {
	Pos     plexer.Position
	FuncDef *FuncDef  `parser:"'async' ( @@"`
	For     *ForStmt  `parser:"  | @@"`
	With    *WithStmt `parser:"  | @@ )"`
}

// Decorated is one or more `@expr NEWLINE` lines followed by a function or
// class definition. The expressions can be any callable: a name, a dotted
// path, or a call. CPython's grammar (3.9+) accepts `@expr` rather than the
// older restricted dotted_name(args) form, so we follow suit.
type Decorated struct {
	Pos        plexer.Position
	Decorators []*Decorator `parser:"@@+"`
	Async      bool         `parser:"( @'async' )?"`
	FuncDef    *FuncDef     `parser:"( @@"`
	ClassDef   *ClassDef    `parser:"| @@ )"`
}

type Decorator struct {
	Pos  plexer.Position
	Expr *Expression `parser:"AT @@ NEWLINE"`
}

// Block is the body of a compound statement. Two shapes:
//
//	def f(): ...                    inline simple statements after the colon
//	def f():\n    body              the standard indented block
//
// The inline form folds one or more SEMI-separated SimpleStmts onto the
// same line as the colon. CPython grammar calls this `simple_stmt` for
// the suite; gopapy reuses SimpleStmts.
type Block struct {
	Pos    plexer.Position
	Body   []*Statement `parser:"  NEWLINE INDENT @@+ DEDENT"`
	Inline *SimpleStmts `parser:"| @@"`
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
	Star bool        `parser:"'except' @STAR?"`
	Type *Expression `parser:"( @@"`
	Name string      `parser:"  ( 'as' @NAME )? )?"`
	Body *Block      `parser:"COLON @@"`
}

type FinallyClause struct {
	Pos  plexer.Position
	Body *Block `parser:"'finally' COLON @@"`
}

type FuncDef struct {
	Pos        plexer.Position
	Name       string       `parser:"'def' @NAME"`
	TypeParams []*TypeParam `parser:"( LBRACK @@ ( COMMA @@ )* COMMA? RBRACK )?"`
	Params     []*Param     `parser:"LPAREN ( @@ ( COMMA @@ )* )? RPAREN"`
	Returns    *Expression  `parser:"( ARROW @@ )?"`
	Body       *Block       `parser:"COLON @@"`
}

// TypeParam is one entry in the bracketed type-parameter list of a def,
// class, or `type` alias (PEP 695). Three shapes:
//
//	NAME                          plain TypeVar
//	NAME : Expression             TypeVar with bound (or constraint tuple)
//	NAME = Expression             TypeVar with default (PEP 696)
//	*NAME                         TypeVarTuple
//	**NAME                        ParamSpec
//
// The variants share a struct; emit picks the AST node based on Kind.
type TypeParam struct {
	Pos     plexer.Position
	Kind    string      `parser:"@( DOUBLESTAR | STAR )?"`
	Name    string      `parser:"@NAME"`
	Bound   *Expression `parser:"( COLON @@ )?"`
	Default *Expression `parser:"( EQ @@ )?"`
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
	// Kind is one of "" (regular), "/" (PEP 570 marker), "*" (vararg or
	// bare-star kwonly marker), or "**" (kwarg). Kept as a string instead
	// of three bools because participle binds at most one capture per
	// field, and we want both the prefix and the name in one Param shape.
	Kind    string      `parser:"@( SLASH | DOUBLESTAR | STAR )?"`
	Name    string      `parser:"@NAME?"`
	Annot   *Expression `parser:"( COLON @@ )?"`
	Default *Expression `parser:"( EQ @@ )?"`
}

type ClassDef struct {
	Pos        plexer.Position
	Name       string       `parser:"'class' @NAME"`
	TypeParams []*TypeParam `parser:"( LBRACK @@ ( COMMA @@ )* COMMA? RBRACK )?"`
	Bases      []*Argument  `parser:"( LPAREN ( @@ ( COMMA @@ )* )? RPAREN )?"`
	Body       *Block       `parser:"COLON @@"`
}

// TypeAliasStmt: PEP 695 `type Name [TypeParams] = Expression`. `type` is a
// soft keyword recognised at statement position only; the participle
// alternation in SimpleStmt tries this before falling back to ExprStmt so
// `type = 1` still parses as an assignment.
type TypeAliasStmt struct {
	Pos        plexer.Position
	Name       string       `parser:"'type' @NAME"`
	TypeParams []*TypeParam `parser:"( LBRACK @@ ( COMMA @@ )* COMMA? RBRACK )?"`
	Value      *Expression  `parser:"EQ @@"`
}
