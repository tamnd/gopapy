package ast

// Transformer is the rewriting counterpart to Visitor (visit.go). Each
// call to Transform is the chance to replace the node it receives:
//
//   - Returning the same node (the receiver of the call) means "keep
//     walking; descend into the children of this node and apply
//     transforms to each child slot in place".
//   - Returning a different non-nil node replaces the original in its
//     parent's child slot. The replacement is *not* itself recursed
//     into — if the caller wants that, they call Apply on the
//     replacement themselves.
//   - Returning nil removes the node. Inside a list slot, the entry
//     drops out of the list. Inside a scalar slot, the field is set
//     to its zero value (typically nil); downstream code that
//     expected a required value may panic later. Match CPython's
//     NodeTransformer: removal is the caller's responsibility.
//
// The shape mirrors CPython `ast.NodeTransformer`. v0.1.10 ships the
// substrate; concrete transformers (constant folding, name renaming,
// etc.) layer on top in user code.
type Transformer interface {
	Transform(n Node) Node
}

// Apply drives t across n in pre-order. For each non-nil node it
// calls t.Transform(n); if Transform returned the same node, Apply
// descends into the children, replacing each child slot with the
// transformed result. The returned value is the (possibly replaced)
// root.
//
// Argument order mirrors Visit(v, n): actor first, target second.
func Apply(t Transformer, n Node) Node {
	if t == nil || isNilNode(n) {
		return n
	}
	next := t.Transform(n)
	if next == nil {
		return nil
	}
	if !sameNode(next, n) {
		// Replacement: do not descend into it. The caller's Transform
		// is the authority on the replacement subtree.
		return next
	}
	transformChildren(n, t)
	return n
}

// transformOne is the per-scalar-slot helper used by transform_gen.go.
// It calls Apply on the slot and re-asserts the result back to the
// slot's compile-time type T. If Transform returns a node whose
// concrete type doesn't satisfy T (e.g. an *FunctionDef where the slot
// is ExprNode), the assertion panics with the standard Go message —
// surfacing the contract violation at the slot, not later inside an
// emitter or printer.
func transformOne[T Node](t Transformer, slot T) T {
	if isNilNode(slot) {
		return slot
	}
	next := Apply(t, slot)
	if next == nil {
		var zero T
		return zero
	}
	return next.(T)
}

// transformList is the per-list-slot helper used by transform_gen.go.
// It applies t to each element; nil-returning elements are removed
// from the result. The original slice is not mutated; transformList
// always allocates a fresh result so callers that hold the old slice
// see a stable snapshot.
func transformList[T Node](t Transformer, slots []T) []T {
	if len(slots) == 0 {
		return slots
	}
	out := make([]T, 0, len(slots))
	for _, s := range slots {
		if isNilNode(s) {
			out = append(out, s)
			continue
		}
		next := Apply(t, s)
		if next == nil {
			continue
		}
		out = append(out, next.(T))
	}
	return out
}

// sameNode reports whether two Node interface values point at the same
// underlying object. Comparing the boxed interfaces directly works
// because every Node is a pointer (interface boxes the type+addr
// pair); a deep equality check would over-match.
func sameNode(a, b Node) bool { return a == b }
