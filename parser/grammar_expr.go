package parser

import (
	plexer "github.com/alecthomas/participle/v2/lexer"
)

// Expression is the top-level expression rule, encompassing the conditional
// expression (`X if C else Y`), lambda, and the walrus assignment expression
// (NAME := expr). Walrus binds looser than the conditional but tighter than
// a bare assignment statement.
type Expression struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Yield  *YieldExpr   `parser:"  @@"`
	Walrus *WalrusExpr  `parser:"| @@"`
	Lambda *Lambda      `parser:"| @@"`
	Body   *Disjunction `parser:"| @@"`
	IfTest *Disjunction `parser:"  ( 'if' @@"`
	IfElse *Expression  `parser:"    'else' @@ )?"`
}

// WalrusExpr is `NAME := expr`. The CPython grammar restricts walrus to
// specific positions (call args, comprehension conditions, parenthesized
// expressions, ...). gopapy follows the leniency of participle: we accept
// walrus anywhere an Expression appears and rely on a downstream pass to
// flag misuse, which mirrors how CPython itself reports the SyntaxError
// only after parse.
type WalrusExpr struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Name   string      `parser:"@NAME WALRUS"`
	Value  *Expression `parser:"@@"`
}

type Lambda struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Params []*LambdaParam `parser:"'lambda' ( @@ ( COMMA @@ )* )? COLON"`
	Body   *Expression    `parser:"@@"`
}

// LambdaParam mirrors Param but without the `: annot` slot — annotations
// are not allowed inside a lambda signature, and accepting them here makes
// the grammar swallow the COLON that ends the lambda header. Default values
// are still allowed.
type LambdaParam struct {
	Pos     plexer.Position
	EndPos  plexer.Position
	Kind    string      `parser:"@( SLASH | DOUBLESTAR | STAR )?"`
	Name    string      `parser:"@NAME?"`
	Default *Expression `parser:"( EQ @@ )?"`
}

// Disjunction = `or` chain of Conjunctions.
type Disjunction struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Conjunction   `parser:"@@"`
	Tail   []*Conjunction `parser:"( 'or' @@ )*"`
}

// Conjunction = `and` chain of Inversions.
type Conjunction struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Inversion   `parser:"@@"`
	Tail   []*Inversion `parser:"( 'and' @@ )*"`
}

// Inversion = `not Inversion` | Comparison.
type Inversion struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Not    bool        `parser:"( @'not'"`
	Inv    *Inversion  `parser:"  @@"`
	Comp   *Comparison `parser:"| @@ )"`
}

// Comparison = BitOr followed by zero or more (cmp_op BitOr).
type Comparison struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *BitOr      `parser:"@@"`
	Tail   []*CmpRight `parser:"@@*"`
}

type CmpRight struct {
	Pos    plexer.Position
	EndPos plexer.Position
	// The `not in` form is its own alternative because both tokens are
	// keywords — ordering matters so participle doesn't treat the `not`
	// as a unary inversion of the right-hand side.
	NotIn bool `parser:"( 'not' @'in'"`
	// `is`, optionally followed by `not`. The IsNot bool gets folded into
	// CmpOp = IsNot in the emitter.
	Is    bool `parser:"| @'is'"`
	IsNot bool `parser:"  @'not'?"`
	// All other comparison ops emit as a single token.
	Op  string `parser:"| @( EQEQ | NE | LE | GE | LT | GT | 'in' ) )"`
	RHS *BitOr `parser:"@@"`
}

// BitOr | BitXor | BitAnd are right-recursive lists folded left in the emitter.
type BitOr struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *BitXor   `parser:"@@"`
	Tail   []*BitXor `parser:"( PIPE @@ )*"`
}
type BitXor struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *BitAnd   `parser:"@@"`
	Tail   []*BitAnd `parser:"( CARET @@ )*"`
}
type BitAnd struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Shift   `parser:"@@"`
	Tail   []*Shift `parser:"( AMP @@ )*"`
}

type Shift struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Sum       `parser:"@@"`
	Tail   []*ShiftOp `parser:"@@*"`
}
type ShiftOp struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Op     string `parser:"@( LSHIFT | RSHIFT )"`
	RHS    *Sum   `parser:"@@"`
}

type Sum struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Term    `parser:"@@"`
	Tail   []*SumOp `parser:"@@*"`
}
type SumOp struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Op     string `parser:"@( PLUS | MINUS )"`
	RHS    *Term  `parser:"@@"`
}

type Term struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Head   *Factor   `parser:"@@"`
	Tail   []*TermOp `parser:"@@*"`
}
type TermOp struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Op     string  `parser:"@( STAR | SLASH | DOUBLESLASH | PERCENT | AT )"`
	RHS    *Factor `parser:"@@"`
}

type Factor struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Unary  string  `parser:"( @( PLUS | MINUS | TILDE )"`
	Inner  *Factor `parser:"  @@"`
	Power  *Power  `parser:"| @@ )"`
}

// Power = AwaitPrimary ('**' Factor)?
type Power struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Await  *AwaitPrimary `parser:"@@"`
	Exp    *Factor       `parser:"( DOUBLESTAR @@ )?"`
}

type AwaitPrimary struct {
	Pos     plexer.Position
	EndPos  plexer.Position
	Await   bool     `parser:"@'await'?"`
	Primary *Primary `parser:"@@"`
}

// Primary is an atom followed by zero or more trailers.
type Primary struct {
	Pos      plexer.Position
	EndPos   plexer.Position
	Atom     *Atom      `parser:"@@"`
	Trailers []*Trailer `parser:"@@*"`
}

type Trailer struct {
	Pos    plexer.Position
	EndPos plexer.Position
	// .NAME
	Attr string `parser:"  DOT @NAME"`
	// (...)
	Call *CallArgs `parser:"| LPAREN @@ RPAREN"`
	// [slices]
	Sub *SubscriptList `parser:"| LBRACK @@ RBRACK"`
}

// CallArgs is the argument list of a call. Two shapes:
//
//	`f(a, b, c)`           regular argument list
//	`f(x for x in xs)`     single bare GeneratorExp; the call parens
//	                       double as the genexp parens. Mixing a genexp
//	                       with other args requires its own parens.
type CallArgs struct {
	Pos    plexer.Position
	EndPos plexer.Position
	First  *Argument   `parser:"( @@"`
	Gen    []*CompFor  `parser:"  ( @@+"`
	Rest   []*Argument `parser:"  | ( COMMA @@ )+ COMMA? | COMMA )? )?"`
}

// Argument covers positional, *star, **double-star, and keyword arguments.
// Kwarg is tried first; if there's no EQ after NAME, participle backtracks
// to Posn (a bare expression).
type Argument struct {
	Pos    plexer.Position
	EndPos plexer.Position
	DStar  *Expression `parser:"  DOUBLESTAR @@"`
	Star   *Expression `parser:"| STAR @@"`
	Kwarg  *KwargPair  `parser:"| @@"`
	Posn   *Expression `parser:"| @@"`
}

type KwargPair struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Name   string      `parser:"@NAME EQ"`
	Value  *Expression `parser:"@@"`
}

// SubscriptList is the contents of a `[ ... ]` trailer. Each Subscript is
// either a bare expression (`a[i]`) or a slice (`a[i:j]`, `a[::s]`, `a[:]`).
// The `!` non-empty modifier on Subscript prevents an empty match (which
// would let SubscriptList spin on no input).
type SubscriptList struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Items  []*Subscript `parser:"@@ ( COMMA @@ )* COMMA?"`
}

// Subscript has three shapes, kept as explicit alternatives so participle
// never sees a branch that can match zero tokens (which trips its no-progress
// guard):
//
//	`*expr`                  Star + Lower
//	`expr` / `expr:...`      Plain (+ optional Slice)
//	`:upper?:step?`          BareSlice (Plain absent — pure colon slice)
type Subscript struct {
	Pos       plexer.Position
	EndPos    plexer.Position
	Star      bool        `parser:"( @STAR"`
	Lower     *Expression `parser:"  @@"`
	Plain     *Expression `parser:"| @@"`
	Slice     *SliceTail  `parser:"  @@?"`
	BareSlice *SliceTail  `parser:"| @@ )"`
}

// SliceTail is `: upper? (: step?)?` — only present if the subscript actually
// has a colon. Lower (on Subscript) is optional, so a pure colon slice like
// `a[:]` parses with Lower=nil and Slice=SliceTail{Upper:nil}.
type SliceTail struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Upper  *Expression `parser:"COLON @@?"`
	Step   *SliceStep  `parser:"@@?"`
}

type SliceStep struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Value  *Expression `parser:"COLON @@?"`
}

type Atom struct {
	Pos      plexer.Position
	EndPos   plexer.Position
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

// ListLit is `[ ... ]`. The body is either empty, a list-of-elements
// (with optional trailing comma), or a comprehension `expr for x in xs ...`.
// The Comp slice being non-empty flips emission from List to ListComp.
type ListLit struct {
	Pos    plexer.Position
	EndPos plexer.Position
	First  *StarOrExpr   `parser:"LBRACK ( @@"`
	Comp   []*CompFor    `parser:"  ( @@+"`
	Rest   []*StarOrExpr `parser:"  | ( COMMA @@ )+ COMMA? | COMMA )? )? RBRACK"`
}

// CompFor is one comprehension clause: optional `async`, then `for target
// in iter`, then any number of `if cond` filters bound to that for-clause.
// Iter and Ifs use Disjunction (not Expression) so a trailing `if` is
// recognised as another filter rather than a conditional expression.
type CompFor struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Async  bool           `parser:"@'async'? 'for'"`
	Target *TargetList    `parser:"@@ 'in'"`
	Iter   *Disjunction   `parser:"@@"`
	Ifs    []*Disjunction `parser:"( 'if' @@ )*"`
}

// StarOrExpr is a single element inside a list/set/tuple literal: either
// a bare expression or `*expr` for iterable unpacking. The emitter wraps
// the latter in a Starred node with Load context.
type StarOrExpr struct {
	Pos    plexer.Position
	EndPos plexer.Position
	Star   *Expression `parser:"  STAR @@"`
	Expr   *Expression `parser:"| @@"`
}

// DictOrSetLit is `{ ... }`. The first item picks dict-vs-set: a `**` or
// a `key: value` pair makes it a dict, anything else is a set. A trailing
// comprehension flips Dict->DictComp or Set->SetComp.
type DictOrSetLit struct {
	Pos    plexer.Position
	EndPos plexer.Position
	First  *DictItemOrExpr   `parser:"LBRACE ( @@"`
	Comp   []*CompFor        `parser:"  ( @@+"`
	Rest   []*DictItemOrExpr `parser:"  | ( COMMA @@ )+ COMMA? | COMMA )? )? RBRACE"`
}

// DictItemOrExpr captures one element inside `{ ... }`. Possibilities:
//
//	`**expr`           dict unpacking (DStar holds the value)
//	`*expr`            set unpacking   (StarSet)
//	`expr : expr`      dict entry      (Key + Value)
//	`expr`             set element     (Key only, Value nil)
//
// The emitter checks fields to decide between Dict and Set and between
// the unpacking and literal forms.
type DictItemOrExpr struct {
	Pos     plexer.Position
	EndPos  plexer.Position
	DStar   *Expression `parser:"  DOUBLESTAR @@"`
	StarSet *Expression `parser:"| STAR @@"`
	Key     *Expression `parser:"| @@"`
	Value   *Expression `parser:"  ( COLON @@ )?"`
}

// ParenLit is `( expr (, expr)* ,? )`. TrailingComma flips on when the
// source ended with a comma — needed to disambiguate `(x)` (a parenthesized
// expression) from `(x,)` (a single-element tuple). Both shapes have one
// element in Elts; only the comma flag tells them apart.
// ParenLit is `( ... )`. Three shapes:
//
//	`(x)`           parenthesized expression: First set, Rest empty, no comma
//	`(x,)` `(a, b)` Tuple: TrailingComma set or Rest non-empty
//	`(x for x in xs)` GeneratorExp: Comp non-empty
type ParenLit struct {
	Pos           plexer.Position
	EndPos        plexer.Position
	First         *StarOrExpr   `parser:"LPAREN ( @@"`
	Comp          []*CompFor    `parser:"  ( @@+"`
	Rest          []*StarOrExpr `parser:"  | ( COMMA @@ )+"`
	TrailingComma bool          `parser:"      @COMMA? | @COMMA )? )? RPAREN"`
}

// YieldExpr = `yield` [`from` expression | star_expressions]
//
// ValRest captures the bare-tuple form (`yield a, b`). The emitter folds
// Val + ValRest into a single Tuple expression, matching CPython's
// `yield_stmt: 'yield' [star_expressions]` shape.
type YieldExpr struct {
	Pos     plexer.Position
	EndPos  plexer.Position
	From    *Expression   `parser:"'yield' ( 'from' @@"`
	Val     *StarOrExpr   `parser:"  | @@"`
	ValRest []*StarOrExpr `parser:"    ( COMMA @@ )* COMMA? )?"`
}
