package parser

// arena holds pre-grown slices for every AST node type. Allocating from a
// slice instead of calling new(T) for each node reduces the number of
// individual heap objects from ~1000 per file to ~77 (one backing-array
// growth event per type after warm-up), which lowers GC pressure.
//
// arenaAlloc appends v to the slice *s and returns a pointer to the new
// element. The pointer remains valid as long as no other append to *s
// triggers a reallocation — which is guaranteed because the caller owns the
// arena and no concurrent mutations occur.
//
// The arena is embedded in the parser struct and is therefore tied to the
// parser's lifetime. Returned *Module and all nested node pointers point into
// the arena slices; they remain valid as long as the parser (or its arena's
// backing arrays) is reachable by the GC, which is ensured because the live
// *Module holds pointers into those arrays.

func arenaAlloc[T any](s *[]T, v T) *T {
	*s = append(*s, v)
	return &(*s)[len(*s)-1]
}

type arena struct {
	aliases         []Alias
	annAssigns      []AnnAssign
	args            []Arg
	argumentsList   []Arguments
	asserts         []Assert
	assigns         []Assign
	asyncFors       []AsyncFor
	asyncFuncDefs   []AsyncFunctionDef
	asyncWiths      []AsyncWith
	attributes      []Attribute
	augAssigns      []AugAssign
	awaits          []Await
	binOps          []BinOp
	boolOps         []BoolOp
	breaks          []Break
	calls           []Call
	classDefs       []ClassDef
	compares        []Compare
	comprehensions  []Comprehension
	constants       []Constant
	continues       []Continue
	deletes         []Delete
	dicts           []Dict
	dictComps       []DictComp
	exceptHandlers  []ExceptHandler
	exprStmts       []ExprStmt
	fors            []For
	formattedValues []FormattedValue
	funcDefs        []FunctionDef
	generatorExps   []GeneratorExp
	globals         []Global
	ifs             []If
	ifExprs         []IfExp
	imports         []Import
	importFroms     []ImportFrom
	interpolations  []Interpolation
	joinedStrs      []JoinedStr
	keywords        []Keyword
	lambdas         []Lambda
	lists           []List
	listComps       []ListComp
	matches         []Match
	matchAsList     []MatchAs
	matchCases      []MatchCase
	matchClasses    []MatchClass
	matchMappings   []MatchMapping
	matchOrs        []MatchOr
	matchSequences  []MatchSequence
	matchSingletons []MatchSingleton
	matchStars      []MatchStar
	matchValues     []MatchValue
	modules         []Module
	names           []Name
	namedExprs      []NamedExpr
	nonlocals       []Nonlocal
	paramSpecs      []ParamSpec
	passes          []Pass
	raises          []Raise
	returns         []Return
	sets            []Set
	setComps        []SetComp
	slices          []Slice
	starredNodes    []Starred
	subscripts      []Subscript
	templateStrs    []TemplateStr
	tries           []Try
	tryStars        []TryStar
	tuples          []Tuple
	typeAliases     []TypeAlias
	typeVars        []TypeVar
	typeVarTuples   []TypeVarTuple
	unaryOps        []UnaryOp
	whiles          []While
	withs           []With
	withItems       []WithItem
	yields          []Yield
	yieldFroms      []YieldFrom

	// exprLists holds contiguous segments of []Expr for assignment target
	// lists. Each plain-assignment statement claims a slice
	// exprLists[start:end]; the segment is valid until arena.reset().
	exprLists []Expr

	// Scratch buffers for non-reentrant list-building call sites.
	// Each is reset at the start of the function that owns it.
	simpleStmtBuf []Stmt   // parseSimpleStmtList
	plainPartsBuf []string // parseStringAtom implicit-concat accumulator
}

// reset truncates all slices to length zero, retaining their backing arrays
// for reuse in the next parse. Only safe to call after the caller is done
// with all pointers into the previous parse's AST.
func (a *arena) reset() {
	a.aliases = a.aliases[:0]
	a.annAssigns = a.annAssigns[:0]
	a.args = a.args[:0]
	a.argumentsList = a.argumentsList[:0]
	a.asserts = a.asserts[:0]
	a.assigns = a.assigns[:0]
	a.asyncFors = a.asyncFors[:0]
	a.asyncFuncDefs = a.asyncFuncDefs[:0]
	a.asyncWiths = a.asyncWiths[:0]
	a.attributes = a.attributes[:0]
	a.augAssigns = a.augAssigns[:0]
	a.awaits = a.awaits[:0]
	a.binOps = a.binOps[:0]
	a.boolOps = a.boolOps[:0]
	a.breaks = a.breaks[:0]
	a.calls = a.calls[:0]
	a.classDefs = a.classDefs[:0]
	a.compares = a.compares[:0]
	a.comprehensions = a.comprehensions[:0]
	a.constants = a.constants[:0]
	a.continues = a.continues[:0]
	a.deletes = a.deletes[:0]
	a.dicts = a.dicts[:0]
	a.dictComps = a.dictComps[:0]
	a.exceptHandlers = a.exceptHandlers[:0]
	a.exprStmts = a.exprStmts[:0]
	a.fors = a.fors[:0]
	a.formattedValues = a.formattedValues[:0]
	a.funcDefs = a.funcDefs[:0]
	a.generatorExps = a.generatorExps[:0]
	a.globals = a.globals[:0]
	a.ifs = a.ifs[:0]
	a.ifExprs = a.ifExprs[:0]
	a.imports = a.imports[:0]
	a.importFroms = a.importFroms[:0]
	a.interpolations = a.interpolations[:0]
	a.joinedStrs = a.joinedStrs[:0]
	a.keywords = a.keywords[:0]
	a.lambdas = a.lambdas[:0]
	a.lists = a.lists[:0]
	a.listComps = a.listComps[:0]
	a.matches = a.matches[:0]
	a.matchAsList = a.matchAsList[:0]
	a.matchCases = a.matchCases[:0]
	a.matchClasses = a.matchClasses[:0]
	a.matchMappings = a.matchMappings[:0]
	a.matchOrs = a.matchOrs[:0]
	a.matchSequences = a.matchSequences[:0]
	a.matchSingletons = a.matchSingletons[:0]
	a.matchStars = a.matchStars[:0]
	a.matchValues = a.matchValues[:0]
	a.modules = a.modules[:0]
	a.names = a.names[:0]
	a.namedExprs = a.namedExprs[:0]
	a.nonlocals = a.nonlocals[:0]
	a.paramSpecs = a.paramSpecs[:0]
	a.passes = a.passes[:0]
	a.raises = a.raises[:0]
	a.returns = a.returns[:0]
	a.sets = a.sets[:0]
	a.setComps = a.setComps[:0]
	a.slices = a.slices[:0]
	a.starredNodes = a.starredNodes[:0]
	a.subscripts = a.subscripts[:0]
	a.templateStrs = a.templateStrs[:0]
	a.tries = a.tries[:0]
	a.tryStars = a.tryStars[:0]
	a.tuples = a.tuples[:0]
	a.typeAliases = a.typeAliases[:0]
	a.typeVars = a.typeVars[:0]
	a.typeVarTuples = a.typeVarTuples[:0]
	a.unaryOps = a.unaryOps[:0]
	a.whiles = a.whiles[:0]
	a.withs = a.withs[:0]
	a.withItems = a.withItems[:0]
	a.yields = a.yields[:0]
	a.yieldFroms = a.yieldFroms[:0]
	a.exprLists = a.exprLists[:0]
	a.simpleStmtBuf = a.simpleStmtBuf[:0]
	a.plainPartsBuf = a.plainPartsBuf[:0]
}
