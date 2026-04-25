def f(x):
    match x:
        case []:
            pass
        case [1, 2, 3]:
            pass
        case [a, b, *rest]:
            use(a, b, rest)
        case [first, *_, last]:
            use(first, last)
        case (a, b):
            use(a, b)
