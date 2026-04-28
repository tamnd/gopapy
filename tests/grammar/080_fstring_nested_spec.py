# Python 3.13+
# f-string with a nested replacement field inside another field's
# format spec (PEP 701 / PEP 3101).
# Python 3.12 early releases (before 3.12.3) appended a trailing Constant('')
# to format_spec JoinedStr values ending in a FormattedValue; later 3.12.x
# and 3.13+ removed it. Skip for 3.12 and below to avoid version ambiguity.
s = f'{x:{y}>10}'
t = f'{value:{width!r}.{precision}}'
u = f'{x:{y:0}}'
