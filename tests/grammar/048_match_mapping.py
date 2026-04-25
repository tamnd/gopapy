def f(x):
    match x:
        case {}:
            pass
        case {"key": value}:
            use(value)
        case {"a": 1, "b": 2}:
            pass
        case {"a": v, **rest}:
            use(v, rest)
