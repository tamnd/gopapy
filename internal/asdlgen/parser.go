// Package asdlgen parses CPython's Parser/Python.asdl and produces Go source
// for the gopapy AST. The ASDL grammar itself is small and stable; the parser
// here recognises only what Python.asdl uses.
//
// Reference: https://www.cs.princeton.edu/research/techreps/TR-554-97
package asdlgen

import (
	"fmt"
	"strings"
	"unicode"
)

// Module is the top-level result: `module NAME { defs }`.
type Module struct {
	Name string
	Defs []*Def
}

// Def is one named type. A sum type has Constructors; a product type has only
// Fields. The Attributes slice (if non-empty) is appended to every constructor
// of a sum type, or to the product type itself.
type Def struct {
	Name         string
	IsProduct    bool
	Constructors []*Constructor
	Fields       []*Field
	Attributes   []*Field
}

// Constructor is one alternative of a sum type, e.g. `BinOp(expr left, ...)`.
type Constructor struct {
	Name   string
	Fields []*Field
}

// Field is `type[?|*|?*] name`.
type Field struct {
	Type   string
	Name   string
	Opt    bool // `?`
	Seq    bool // `*`
	OptSeq bool // `?*` (list whose elements may be nil)
}

// Parse reads a Python.asdl source string and returns the module.
func Parse(src string) (*Module, error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	return p.parseModule()
}

// ---------------------------------------------------------------------------
// Tokeniser
// ---------------------------------------------------------------------------

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tLBrace
	tRBrace
	tLParen
	tRParen
	tComma
	tEq
	tPipe
	tStar
	tQuestion
)

type token struct {
	kind tokKind
	val  string
	pos  int // byte offset for error messages
}

func tokenize(src string) ([]token, error) {
	var out []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			// comment to end of line
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '{':
			out = append(out, token{tLBrace, "{", i})
			i++
		case c == '}':
			out = append(out, token{tRBrace, "}", i})
			i++
		case c == '(':
			out = append(out, token{tLParen, "(", i})
			i++
		case c == ')':
			out = append(out, token{tRParen, ")", i})
			i++
		case c == ',':
			out = append(out, token{tComma, ",", i})
			i++
		case c == '=':
			out = append(out, token{tEq, "=", i})
			i++
		case c == '|':
			out = append(out, token{tPipe, "|", i})
			i++
		case c == '*':
			out = append(out, token{tStar, "*", i})
			i++
		case c == '?':
			out = append(out, token{tQuestion, "?", i})
			i++
		case isIdentStart(rune(c)):
			start := i
			for i < len(src) && isIdentPart(rune(src[i])) {
				i++
			}
			out = append(out, token{tIdent, src[start:i], start})
		default:
			return nil, fmt.Errorf("asdl: unexpected character %q at offset %d", c, i)
		}
	}
	out = append(out, token{tEOF, "", len(src)})
	return out, nil
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token        { return p.toks[p.pos] }
func (p *parser) next() token        { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) eat(k tokKind) bool { return p.peek().kind == k }

func (p *parser) expect(k tokKind, what string) (token, error) {
	t := p.peek()
	if t.kind != k {
		return token{}, fmt.Errorf("asdl: expected %s, got %q at offset %d", what, t.val, t.pos)
	}
	p.pos++
	return t, nil
}

func (p *parser) parseModule() (*Module, error) {
	if t := p.next(); t.kind != tIdent || t.val != "module" {
		return nil, fmt.Errorf("asdl: expected 'module' at start, got %q", t.val)
	}
	name, err := p.expect(tIdent, "module name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tLBrace, "'{'"); err != nil {
		return nil, err
	}
	m := &Module{Name: name.val}
	for !p.eat(tRBrace) {
		if p.eat(tEOF) {
			return nil, fmt.Errorf("asdl: unexpected EOF inside module")
		}
		d, err := p.parseDef()
		if err != nil {
			return nil, err
		}
		m.Defs = append(m.Defs, d)
	}
	p.next() // consume '}'
	return m, nil
}

func (p *parser) parseDef() (*Def, error) {
	name, err := p.expect(tIdent, "type name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tEq, "'='"); err != nil {
		return nil, err
	}
	d := &Def{Name: name.val}
	if p.eat(tLParen) {
		// product type: `name = (field, field, ...)`
		d.IsProduct = true
		fields, err := p.parseFieldList()
		if err != nil {
			return nil, err
		}
		d.Fields = fields
	} else {
		// sum type: list of constructors separated by '|'
		c, err := p.parseConstructor()
		if err != nil {
			return nil, err
		}
		d.Constructors = append(d.Constructors, c)
		for p.eat(tPipe) {
			p.next()
			c, err := p.parseConstructor()
			if err != nil {
				return nil, err
			}
			d.Constructors = append(d.Constructors, c)
		}
	}
	// optional `attributes ( ... )`
	if p.eat(tIdent) && p.peek().val == "attributes" {
		p.next() // consume 'attributes'
		fields, err := p.parseFieldList()
		if err != nil {
			return nil, err
		}
		d.Attributes = fields
	}
	return d, nil
}

func (p *parser) parseConstructor() (*Constructor, error) {
	name, err := p.expect(tIdent, "constructor name")
	if err != nil {
		return nil, err
	}
	c := &Constructor{Name: name.val}
	if p.eat(tLParen) {
		fields, err := p.parseFieldList()
		if err != nil {
			return nil, err
		}
		c.Fields = fields
	}
	return c, nil
}

// parseFieldList consumes `( field , field , ... )` including the parens.
func (p *parser) parseFieldList() ([]*Field, error) {
	if _, err := p.expect(tLParen, "'('"); err != nil {
		return nil, err
	}
	var fields []*Field
	if !p.eat(tRParen) {
		for {
			f, err := p.parseField()
			if err != nil {
				return nil, err
			}
			fields = append(fields, f)
			if p.eat(tComma) {
				p.next()
				continue
			}
			break
		}
	}
	if _, err := p.expect(tRParen, "')'"); err != nil {
		return nil, err
	}
	return fields, nil
}

// parseField reads `type[?|*|?*] name`. The mark order in Python.asdl is
// `type? name`, `type* name`, or `type?* name`.
func (p *parser) parseField() (*Field, error) {
	typ, err := p.expect(tIdent, "field type")
	if err != nil {
		return nil, err
	}
	f := &Field{Type: typ.val}
	// optional `?` then optional `*`, OR just `*`
	if p.eat(tQuestion) {
		p.next()
		if p.eat(tStar) {
			p.next()
			f.OptSeq = true
		} else {
			f.Opt = true
		}
	} else if p.eat(tStar) {
		p.next()
		f.Seq = true
	}
	name, err := p.expect(tIdent, "field name")
	if err != nil {
		return nil, err
	}
	f.Name = name.val
	return f, nil
}

// ---------------------------------------------------------------------------
// Helpers used by the generator
// ---------------------------------------------------------------------------

// IsBuiltin reports whether t is one of the four ASDL primitive types.
func IsBuiltin(t string) bool {
	switch t {
	case "identifier", "int", "string", "constant":
		return true
	}
	return false
}

// String renders a field back into ASDL syntax, useful for diagnostics.
func (f *Field) String() string {
	suf := ""
	switch {
	case f.OptSeq:
		suf = "?*"
	case f.Opt:
		suf = "?"
	case f.Seq:
		suf = "*"
	}
	return strings.Join([]string{f.Type + suf, f.Name}, " ")
}
