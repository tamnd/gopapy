# Python 3.10+: match/case statement — as (PEP 634)
def f(x):
    match x:
        case [1, 2] as pair:
            use(pair)
        case Point(x=0) as origin:
            use(origin)
        case 1 | 2 as small:
            use(small)
