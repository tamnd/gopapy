package parser

import (
	plexer "github.com/alecthomas/participle/v2/lexer"
)

// Expression is the top-level expression rule, encompassing the conditional
// expression (`X if C else Y`) and lambda. Lower precedence than disjunction.
type Expression struct {
	Pos    plexer.Position
	Lambda *Lambda      `parser:"  @@"`
	Body   *Disjunction `parser:"| @@"`
	IfTest *Disjunction `parser:"  ( 'if' @@"`
	IfElse *Expression  `parser:"    'else' @@ )?"`
}

type Lambda struct {
	Pos    plexer.Position
	Params []*Param    `parser:"'lambda' ( @@ ( COMMA @@ )* )? COLON"`
	Body   *Expression `parser:"@@"`
}

// Disjunction = `or` chain of Conjunctions.
type Disjunction struct {
	Pos  plexer.Position
	Head *Conjunction   `parser:"@@"`
	Tail []*Conjunction `parser:"( 'or' @@ )*"`
}

// Conjunction = `and` chain of Inversions.
type Conjunction struct {
	Pos  plexer.Position
	Head *Inversion   `parser:"@@"`
	Tail []*Inversion `parser:"( 'and' @@ )*"`
}

// Inversion = `not Inversion` | Comparison.
type Inversion struct {
	Pos  plexer.Position
	Not  bool        `parser:"( @'not'"`
	Inv  *Inversion  `parser:"  @@"`
	Comp *Comparison `parser:"| @@ )"`
}

// Comparison = BitOr followed by zero or more (cmp_op BitOr).
type Comparison struct {
	Pos  plexer.Position
	Head *BitOr      `parser:"@@"`
	Tail []*CmpRight `parser:"@@*"`
}

type CmpRight struct {
	Pos plexer.Position
	// The `not in` form is its own alternative because both tokens are
	// keywords — ordering matters so participle doesn't treat the `not`
	// as a unary inversion of the right-hand side.
	NotIn bool   `parser:"( 'not' @'in'"`
	// `is`, optionally followed by `not`. The IsNot bool gets folded into
	// CmpOp = IsNot in the emitter.
	Is    bool   `parser:"| @'is'"`
	IsNot bool   `parser:"  @'not'?"`
	// All other comparison ops emit as a single token.
	Op    string `parser:"| @( EQEQ | NE | LE | GE | LT | GT | 'in' ) )"`
	RHS   *BitOr `parser:"@@"`
}

// BitOr | BitXor | BitAnd are right-recursive lists folded left in the emitter.
type BitOr struct {
	Pos  plexer.Position
	Head *BitXor   `parser:"@@"`
	Tail []*BitXor `parser:"( PIPE @@ )*"`
}
type BitXor struct {
	Pos  plexer.Position
	Head *BitAnd   `parser:"@@"`
	Tail []*BitAnd `parser:"( CARET @@ )*"`
}
type BitAnd struct {
	Pos  plexer.Position
	Head *Shift   `parser:"@@"`
	Tail []*Shift `parser:"( AMP @@ )*"`
}

type Shift struct {
	Pos  plexer.Position
	Head *Sum       `parser:"@@"`
	Tail []*ShiftOp `parser:"@@*"`
}
type ShiftOp struct {
	Pos plexer.Position
	Op  string `parser:"@( LSHIFT | RSHIFT )"`
	RHS *Sum   `parser:"@@"`
}

type Sum struct {
	Pos  plexer.Position
	Head *Term    `parser:"@@"`
	Tail []*SumOp `parser:"@@*"`
}
type SumOp struct {
	Pos plexer.Position
	Op  string `parser:"@( PLUS | MINUS )"`
	RHS *Term  `parser:"@@"`
}

type Term struct {
	Pos  plexer.Position
	Head *Factor   `parser:"@@"`
	Tail []*TermOp `parser:"@@*"`
}
type TermOp struct {
	Pos plexer.Position
	Op  string  `parser:"@( STAR | SLASH | DOUBLESLASH | PERCENT | AT )"`
	RHS *Factor `parser:"@@"`
}

type Factor struct {
	Pos   plexer.Position
	Unary string  `parser:"( @( PLUS | MINUS | TILDE )"`
	Inner *Factor `parser:"  @@"`
	Power *Power  `parser:"| @@ )"`
}

// Power = AwaitPrimary ('**' Factor)?
type Power struct {
	Pos   plexer.Position
	Await *AwaitPrimary `parser:"@@"`
	Exp   *Factor       `parser:"( DOUBLESTAR @@ )?"`
}

type AwaitPrimary struct {
	Pos     plexer.Position
	Await   bool     `parser:"@'await'?"`
	Primary *Primary `parser:"@@"`
}

// Primary is an atom followed by zero or more trailers.
type Primary struct {
	Pos      plexer.Position
	Atom     *Atom      `parser:"@@"`
	Trailers []*Trailer `parser:"@@*"`
}

type Trailer struct {
	Pos plexer.Position
	// .NAME
	Attr string `parser:"  DOT @NAME"`
	// (...)
	Call *CallArgs `parser:"| LPAREN @@ RPAREN"`
	// [slices]
	Sub *SubscriptList `parser:"| LBRACK @@ RBRACK"`
}

type CallArgs struct {
	Pos  plexer.Position
	Args []*Argument `parser:"( @@ ( COMMA @@ )* COMMA? )?"`
}

// Argument covers positional, *star, **double-star, and keyword arguments.
// Kwarg is tried first; if there's no EQ after NAME, participle backtracks
// to Posn (a bare expression).
type Argument struct {
	Pos   plexer.Position
	DStar *Expression `parser:"  DOUBLESTAR @@"`
	Star  *Expression `parser:"| STAR @@"`
	Kwarg *KwargPair  `parser:"| @@"`
	Posn  *Expression `parser:"| @@"`
}

type KwargPair struct {
	Pos   plexer.Position
	Name  string      `parser:"@NAME EQ"`
	Value *Expression `parser:"@@"`
}

// SubscriptList is the contents of a `[ ... ]` trailer. Each Subscript is
// either a bare expression (`a[i]`) or a slice (`a[i:j]`, `a[::s]`, `a[:]`).
// The `!` non-empty modifier on Subscript prevents an empty match (which
// would let SubscriptList spin on no input).
type SubscriptList struct {
	Pos   plexer.Position
	Items []*Subscript `parser:"@@ ( COMMA @@ )* COMMA?"`
}

type Subscript struct {
	Pos   plexer.Position
	Lower *Expression `parser:"( @@?"`
	Slice *SliceTail  `parser:"  @@? )!"`
}

// SliceTail is `: upper? (: step?)?` — only present if the subscript actually
// has a colon. Lower (on Subscript) is optional, so a pure colon slice like
// `a[:]` parses with Lower=nil and Slice=SliceTail{Upper:nil}.
type SliceTail struct {
	Pos   plexer.Position
	Upper *Expression `parser:"COLON @@?"`
	Step  *SliceStep  `parser:"@@?"`
}

type SliceStep struct {
	Pos   plexer.Position
	Value *Expression `parser:"COLON @@?"`
}

type Atom struct {
	Pos      plexer.Position
	Name     string        `parser:"  @NAME"`
	Number   string        `parser:"| @NUMBER"`
	String   []string      `parser:"| @STRING+"`
	True     bool          `parser:"| @'True'"`
	False_   bool          `parser:"| @'False'"`
	None     bool          `parser:"| @'None'"`
	Ellipsis bool          `parser:"| @ELLIPSIS"`
	List     *ListLit      `parser:"| @@"`
	Dict     *DictOrSetLit `parser:"| @@"`
	Paren    *ParenLit     `parser:"| @@"`
}

type ListLit struct {
	Pos  plexer.Position
	Elts []*Expression `parser:"LBRACK ( @@ ( COMMA @@ )* COMMA? )? RBRACK"`
}

type DictOrSetLit struct {
	Pos   plexer.Position
	First *DictItemOrExpr   `parser:"LBRACE ( @@"`
	Rest  []*DictItemOrExpr `parser:"  ( COMMA @@ )* COMMA? )? RBRACE"`
}

// DictItemOrExpr is `expr COLON expr` (dict entry) or just `expr` (set elt).
// The emitter checks Value to disambiguate.
type DictItemOrExpr struct {
	Pos   plexer.Position
	Key   *Expression `parser:"@@"`
	Value *Expression `parser:"( COLON @@ )?"`
}

type ParenLit struct {
	Pos  plexer.Position
	Elts []*Expression `parser:"LPAREN ( @@ ( COMMA @@ )* COMMA? )? RPAREN"`
}

// YieldExpr = `yield` [`from` expression | star_expressions]
type YieldExpr struct {
	Pos  plexer.Position
	From *Expression `parser:"'yield' ( 'from' @@"`
	Val  *Expression `parser:"  | @@ )?"`
}
