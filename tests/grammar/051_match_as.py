def f(x):
    match x:
        case [1, 2] as pair:
            use(pair)
        case Point(x=0) as origin:
            use(origin)
        case 1 | 2 as small:
            use(small)
