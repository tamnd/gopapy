# Python 3.13+: type parameter defaults (PEP 696)
type Alias[T = int] = list[T]

def f[T = str](x: T) -> T:
    return x

class C[T = bytes]:
    pass
