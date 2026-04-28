# µ (U+00B5 MICRO SIGN) normalizes to μ (U+03BC) under NFKC.
# CPython stores the NFKC form in the AST; gopapy must match.
µ = 1
assert µ == 1
