# `for x, in ...:` — single-element tuple target via trailing comma.
for x, in [(1,), (2,), (3,)]:
    pass

for x, y, in [(1, 2)]:
    pass
