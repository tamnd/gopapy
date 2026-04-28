# Python 3.10+
with (open("a")):
    pass

with (open("a")), (open("b")):
    pass

with (open("a") as f):
    pass

with (open("a")) as f:
    pass

with (a, *b):
    pass

with (a, (b, *c)):
    pass

with (a for b in c):
    pass

with (a, (b for c in d)):
    pass
