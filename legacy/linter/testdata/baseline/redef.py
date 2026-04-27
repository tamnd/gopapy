"""Redefinition — F811 fixtures."""


def helper():
    return 1


def helper():  # F811: redefined without being used
    return 2


class C:
    def m(self):
        return 1

    def m(self):  # F811
        return 2


def with_use():
    x = 1
    print(x)
    x = 2
    return x


def dead_store():
    x = 1
    x = 2  # F811
    return x


print(helper(), with_use(), dead_store(), C().m())
