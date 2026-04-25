def f(x):
    match x:
        case n if n > 0:
            pass
        case [a, b] if a == b:
            pass
        case Point(x=v) if v != 0:
            use(v)
