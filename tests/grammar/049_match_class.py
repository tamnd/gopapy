# Python 3.10+: match/case statement — class (PEP 634)
def f(x):
    match x:
        case Point():
            pass
        case Point(0, 0):
            pass
        case Point(x=0, y=0):
            pass
        case Point(x, y=0):
            use(x)
        case mod.Color(value=v):
            use(v)
