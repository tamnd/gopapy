# Python 3.13+: type parameter defaults (PEP 696) — `[T = int]` requires 3.13
def bounded[T: int](x: T) -> T:
    return x

def constrained[T: (int, str)](x: T) -> T:
    return x

def with_default[T = int](x: T) -> T:
    return x

def varargs[*Ts](xs):
    return xs

def paramspec[**P](f) -> None:
    pass

def mixed[T: int, *Ts, **P](x: T) -> None:
    pass

type Alias[T: int, *Ts, **P] = int
