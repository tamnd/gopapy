# Python 3.11+: except* ExceptionGroup (PEP 654)
try:
    pass
except* ValueError as eg:
    pass
except* (TypeError, KeyError) as eg:
    pass
