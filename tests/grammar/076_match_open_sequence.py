# Python 3.10+: match/case statement — open sequence (PEP 634)
# PEP 634: `case 0, *rest:` is an open (paren-less) sequence pattern.
match xs:
    case 0, *rest:
        pass
    case *head, 9:
        pass
    case 1, 2,:
        pass
