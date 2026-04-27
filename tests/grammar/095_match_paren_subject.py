# Python 3.10+: match/case statement — paren subject (PEP 634)
def f(v):
    match ():
        case ():
            pass
    match (v,):
        case (x,):
            pass
    match (v, v):
        case (x, y):
            pass
