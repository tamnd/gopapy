# Python 3.13+
# Python 3.12 early releases (before 3.12.3) appended a trailing Constant('')
# to format_spec JoinedStr values ending in a FormattedValue; later 3.12.x
# and 3.13+ removed it. Skip for 3.12 and below to avoid version ambiguity.
x = 3.14159
width = 10
prec = 3
a = f"{x:>{width}}"
b = f"{x:>{width}.{prec}f}"
c = f"{x:{width}.{prec}f}"
