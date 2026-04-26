package ast

import (
	"fmt"
	"strings"

	"github.com/tamnd/gopapy/v1/parser"
)

// emitTString folds an implicitly-concatenated run of string literals
// (where at least one is a t-string) into a TemplateStr. PEP 750.
//
// Mirrors emitFString in shape: literal text becomes Constant, `{expr}`
// chunks become Interpolation. The Interpolation node carries the
// original expression source text in its Str field, which is the main
// thing distinguishing it from FormattedValue.
func emitTString(p Pos, parts []string) ExprNode {
	var values []ExprNode
	var lit strings.Builder
	flush := func() {
		if lit.Len() == 0 {
			return
		}
		values = append(values, &Constant{Pos: p, Value: ConstantValue{Kind: ConstantStr, Str: lit.String()}})
		lit.Reset()
	}
	for _, raw := range parts {
		body, isInterp := stripStringQuotes(raw)
		if !isInterp {
			lit.WriteString(decodeEscapes(body))
			continue
		}
		isRaw := isRawFString(getStringPrefix(raw))
		body = applyEscapes(body, isRaw)
		i := 0
		for i < len(body) {
			c := body[i]
			if c == '{' {
				if i+1 < len(body) && body[i+1] == '{' {
					lit.WriteByte('{')
					i += 2
					continue
				}
				flush()
				end, node := scanInterpolationT(p, body, i+1, isRaw)
				if node != nil {
					values = append(values, node)
				}
				i = end
				continue
			}
			if c == '}' {
				if i+1 < len(body) && body[i+1] == '}' {
					lit.WriteByte('}')
					i += 2
					continue
				}
				i++
				continue
			}
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return &TemplateStr{Pos: p, Values: values}
}

// scanInterpolationT is a copy of scanInterpolation that produces an
// Interpolation node instead of a FormattedValue. The body slice for
// Str is captured before any trimming so it round-trips against
// CPython's ast.dump output.
func scanInterpolationT(p Pos, body string, i int, raw bool) (int, ExprNode) {
	depth := 0
	exprStart := i
	exprEnd := -1
	convStart := -1
	specStart := -1
	for i < len(body) {
		c := body[i]
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']':
			depth--
		case '\'', '"':
			i = skipString(body, i)
			continue
		case '!':
			if depth == 0 && exprEnd < 0 && i+1 < len(body) && body[i+1] != '=' {
				exprEnd = i
				convStart = i + 1
			}
		case ':':
			if depth == 0 && specStart < 0 {
				if exprEnd < 0 {
					exprEnd = i
				}
				specStart = i + 1
			}
		case '}':
			if depth > 0 {
				depth--
				break
			}
			if exprEnd < 0 {
				exprEnd = i
			}
			node := buildInterpolation(p, body[exprStart:exprEnd], convText(body, convStart, specStart, i), specText(body, specStart, i), raw)
			return i + 1, node
		}
		i++
	}
	return len(body), nil
}

func buildInterpolation(p Pos, exprText, conv, spec string, raw bool) ExprNode {
	srcText := strings.TrimSpace(exprText)
	if strings.HasSuffix(srcText, "=") {
		srcText = strings.TrimSpace(srcText[:len(srcText)-1])
	}
	if srcText == "" {
		return nil
	}
	expr, err := parser.ParseExpression(srcText)
	if err != nil {
		return &Name{Pos: p, Id: fmt.Sprintf("<tstring-error:%v>", err), Ctx: &Load{}}
	}
	val := emitExpr(expr)
	node := &Interpolation{
		Pos:        p,
		Value:      val,
		Str:        ConstantValue{Kind: ConstantStr, Str: srcText},
		Conversion: conversionOrd(conv),
	}
	if spec != "" {
		node.FormatSpec = &JoinedStr{Pos: p, Values: parseFStringBody(p, applyEscapes(spec, raw), raw)}
	}
	return node
}
