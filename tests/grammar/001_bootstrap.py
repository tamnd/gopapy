pass

1 + 2

x = 1

a = 1 + 2 * 3 - 4
b = 2 ** 3 ** 4
c = -a + +b

x = a < b <= c
y = a is not b
z = a not in b

if a:
    x
elif b:
    y
else:
    z

f(1, x=2, *args, **kwargs)

a = [1, 2, 3]
b = (1, 2, 3)
c = {1, 2, 3}
d = {1: 2, 3: 4}

a[0]
a[1:2]
a[::2]
a[1:2, 3]

import os
import os.path as p
from . import a
from ..pkg import b, c as cc

def f(a, b=1, *args, c, d=2, **kwargs):
    return a + b

class C(B, metaclass=M):
    x = 1
    def m(self):
        pass

a = 'hello'
b = "world"
c = b'\x00bytes'

try:
    a
except ValueError as e:
    b
except:
    c
else:
    d
finally:
    e
