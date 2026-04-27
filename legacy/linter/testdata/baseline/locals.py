"""Locals — F841 fixtures."""


def f():
    x = 1  # F841
    return 2


def g():
    y = 1
    return y


def h(items):
    for item in items:  # for-target exempt
        print(1)
    with open("x") as fh:  # with-target exempt
        return fh.read()


def i():
    try:
        return 1
    except Exception as e:  # except-target exempt
        return 0


def j():
    x = 1
    x += 1  # bind+use, no F841
    return x


def k():
    _ = expensive_call()  # underscore exempt
    return 1


def expensive_call():
    return 0


print(f(), g(), h([1]), i(), j(), k())
