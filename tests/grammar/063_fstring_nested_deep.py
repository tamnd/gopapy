# Python 3.14+: deeply nested f-strings and t-string mixing require 3.14
a = f"{f"{f"{x}"}"}"
b = t"{f"{x}"}"
c = f"{t"{x}"}"
