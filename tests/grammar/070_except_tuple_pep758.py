# PEP 758: `except A, B:` (no parens around the exception-type tuple).
try:
    pass
except ValueError, TypeError:
    pass

try:
    pass
except* ValueError, TypeError:
    pass
