try:
    do()
except* ValueError as eg:
    handle(eg)
except* (TypeError, KeyError) as eg:
    handle(eg)
except* Exception:
    pass
