# A dict literal inside an f-string interpolation: the inner `:` is the
# dict key/value separator, not the start of a format spec.
s = f'expr={ {x: y for x, y in [(1, 2), ]} }'
t = f'{ {1: 2, 3: 4} }'
