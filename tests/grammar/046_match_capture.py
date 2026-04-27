# Python 3.10+: match/case statement — capture (PEP 634)
def f(x):
    match x:
        case y:
            use(y)
        case Color.RED:
            pass
        case mod.Color.GREEN:
            pass
