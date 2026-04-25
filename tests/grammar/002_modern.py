if (n := len(data)) > 10:
    print(n)
while chunk := stream.read():
    handle(chunk)
total = (count := 0) + count

a = [1, *xs, 2]
b = (*xs, *ys)
c = {*xs, 4}
d = {"a": 1, **extra}

a = "\a\b\f\v"
b = "\101\102\103"
c = "é 中"
d = "\U0001F600"
e = "line1 \
line2"

@app.route("/")
def index():
    return "ok"

@trace
@cache(ttl=60)
class Service:
    pass

with open("a") as f:
    f.read()
with open("a") as f, open("b") as g:
    pass
with (open("a") as f, open("b") as g):
    pass

def posonly(a, b, /, c, d=1, *, e, f=2):
    return a + b + c + d + e + f

a = [x * 2 for x in range(10)]
b = [x for x in xs if x > 0]
c = [(x, y) for x in xs for y in ys if x != y]

a = {k: v for k, v in items.items()}
b = {x: x * x for x in range(5) if x}

a = {x for x in xs}
b = {x % 5 for x in range(20) if x > 1}

a = (x for x in xs)
total = sum(x * x for x in range(10))
pairs = list((x, y) for x in xs for y in ys)

async def fetch(url):
    async with session.get(url) as r:
        return await r.text()

async def gather(stream):
    async for chunk in stream:
        yield chunk

@trace
async def slow():
    await asyncio.sleep(1)

name = "world"
a = f"hello {name}"
b = f"{1 + 2}"
c = f"{x} and {y}"
d = f"plain"
e = f""

a = f"{x:>5}"
b = f"{val:.3f}"
c = f"{n:0{width}d}"

a = f"{x!r}"
b = f"{x!s}"
c = f"{x!a}"
d = f"{x!r:>10}"

a = f"{{literal}}"
b = f"{{ {x} }}"
c = "plain " f"{x}" " end"

a = (x,)
b = (1, 2)
c = (x)
empty = ()
