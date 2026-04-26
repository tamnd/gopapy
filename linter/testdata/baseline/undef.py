# Fixture for F821 (undefined name).

import os

print(os.getcwd())


def bad():
    return prnt(missing_name)


class C:
    attr = 1

    def m(self):
        return attr  # class attr not visible from method
