a = (1 + 2) * 3
b = 2 ** 3 ** 4
c = -x ** 2
d = (a if b else c) + 1
e = not (a == b)
f = a < b < c
g = (x := 1) + 2
h = lambda x, y=1, *args, z, **kw: x + y + z
m = a[1:2:3, ..., ::-1]
n = [x for x in xs if x for y in ys]
o = {k: v for k, v in items}
p = (x, y, *rest)
q = [*xs, *ys]
r = f'hello {name!r} {value:>{width}.{prec}f}'
s = f'{{literal}} {x}'
t = a or b and not c
u = func(*args, key=val, **kw)
v = obj.attr.method(1)[2]
