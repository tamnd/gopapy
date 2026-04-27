# Python 3.9+: starred elements in implicit-tuple for-loop iterator
# `for x in *a, *b, *c:` — starred elements in the implicit-tuple iter.
a = [1]
b = [2]
c = [3]
for x in *a, *b, *c:
    pass
