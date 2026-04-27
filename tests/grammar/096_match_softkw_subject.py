# Python 3.10+: match/case statement — softkw subject (PEP 634)
def f(match, case, type):
    match match:
        case 1:
            pass
    match case:
        case 2:
            pass
    match type:
        case 3:
            pass
