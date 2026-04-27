# Python 3.11+: starred expression in subscript (PEP 646)
a: tuple[*Ts]
b: Callable[[*Args], R]
c: dict[str, *Vs]
d: tuple[int, *Ts, str]
