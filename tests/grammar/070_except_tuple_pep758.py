# Python 3.14+: PEP 758 except A, B: without parens (not valid in 3.13 and below).
try:
    pass
except ValueError, TypeError:
    pass

try:
    pass
except* ValueError, TypeError:
    pass
