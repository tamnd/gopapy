# Python 3.10+: match/case statement — basic (PEP 634)
def f(x):
    match x:
        case 0:
            pass
        case 1:
            pass
        case "hello":
            pass
        case True:
            pass
        case None:
            pass
        case _:
            pass
