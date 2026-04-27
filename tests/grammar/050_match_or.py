# Python 3.10+: match/case statement — or (PEP 634)
def f(x):
    match x:
        case 1 | 2 | 3:
            pass
        case "a" | "b":
            pass
        case [1] | [2]:
            pass
