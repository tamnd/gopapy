async def f(it):
    return [x async for x in it]
async def g(it):
    return {x async for x in it}
async def h(it):
    return {x: x + 1 async for x in it}
async def i(it):
    return (x async for x in it)
async def j(a, b):
    return [x + y async for x in a for y in b]
async def k(it):
    return [x async for x in it if x > 0]
async def m(a, b):
    return [x async for x in a async for y in b]
