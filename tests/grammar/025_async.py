async def fetch(url):
    async with session.get(url) as r:
        return await r.text()

async def gather(stream):
    async for chunk in stream:
        yield chunk

@trace
async def slow():
    await asyncio.sleep(1)
