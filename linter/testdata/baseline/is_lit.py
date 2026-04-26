# F632 fixtures: `is` / `is not` against a literal value.

x = 1

if x is 1:           # F632
    pass
if x is "foo":       # F632
    pass
if x is True:        # F632
    pass
if x is not 2:       # F632
    pass
if x is (1, 2):      # F632
    pass
if x is -1:          # F632
    pass

# Negative cases — these must stay silent.
if x is None:
    pass
if x is not None:
    pass
y = 1
if x is y:
    pass
if x == 1:
    pass
