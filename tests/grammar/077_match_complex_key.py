# Python 3.10+: match/case statement — complex key (PEP 634)
# Match mapping pattern with a complex-literal key (`a + bj`, `a - bj`).
match d:
    case {-0-0j: x}:
        pass
    case {1+2j: y}:
        pass
