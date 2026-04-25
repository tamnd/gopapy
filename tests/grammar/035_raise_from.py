def reraise(e):
    raise
    raise ValueError("bad")
    raise ValueError("bad") from e
    raise RuntimeError("wrap") from None
