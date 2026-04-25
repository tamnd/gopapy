# f-string with a nested replacement field inside another field's
# format spec (PEP 701 / PEP 3101).
s = f'{x:{y}>10}'
t = f'{value:{width!r}.{precision}}'
u = f'{x:{y:0}}'
