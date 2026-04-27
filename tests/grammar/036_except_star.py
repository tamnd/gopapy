# Python 3.11+: except* ExceptionGroup handler (PEP 654)
try:
    do()
except* ValueError as eg:
    handle(eg)
except* (TypeError, KeyError) as eg:
    handle(eg)
except* Exception:
    pass
