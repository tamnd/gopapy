package ast

import "reflect"

// Visitor is the typed-actor counterpart to the function-style Walk
// already exposed in this package. CPython's `ast.NodeVisitor` is the
// shape this mirrors: each call to Visit returns the visitor that
// should walk the children, so a single Visitor implementation can
// (a) prune a subtree by returning nil, (b) keep walking with itself
// by returning the receiver, or (c) hand a different visitor to a
// subtree by returning a new value.
//
// The function form (Walk + a closure) is the convenient one-liner
// when no per-type behavior is needed; this Visitor shape is the
// substrate analyzers should reach for once they want stateful or
// per-type dispatch.
type Visitor interface {
	Visit(n Node) Visitor
}

// Visit drives v across n in depth-first, pre-order traversal. For
// each non-nil node, the visitor is asked which visitor (if any) to
// use for the children. Argument order matches `io.Copy(dst, src)`:
// the actor first, the target second.
func Visit(v Visitor, n Node) {
	if v == nil || isNilNode(n) {
		return
	}
	next := v.Visit(n)
	if next == nil {
		return
	}
	walkChildren(n, func(c Node) bool {
		Visit(next, c)
		return false
	})
}

// WalkPreorder calls fn for every non-nil node in n in depth-first
// pre-order. Unlike Walk, fn cannot prune the descent — use Walk or
// Visit for that. The convenience is for the common "do X for every
// node" loop where a `return true` in every branch would just be noise.
func WalkPreorder(n Node, fn func(Node)) {
	if isNilNode(n) {
		return
	}
	Walk(n, func(c Node) bool {
		if isNilNode(c) {
			return false
		}
		fn(c)
		return true
	})
}

// WalkPostorder calls fn for every non-nil node in n in depth-first
// post-order: every descendant is visited before its parent. Useful
// for analyses that aggregate child results into the parent (constant
// folding, free-variable computation, etc.).
func WalkPostorder(n Node, fn func(Node)) {
	if isNilNode(n) {
		return
	}
	walkChildren(n, func(c Node) bool {
		WalkPostorder(c, fn)
		return false
	})
	fn(n)
}

// isNilNode catches both untyped-nil interfaces and typed-nil pointer
// interfaces. The generated walkChildren reads concrete fields like
// `n.Annotation` directly; passing a `(*Arg)(nil)` to it segfaults.
// The bare `n == nil` check is true only when both type and value are
// nil, so we need a reflect-aware guard for the typed case.
func isNilNode(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	}
	return false
}
