# Python 3.9+: generalized decorators (PEP 614)
decorators = [lambda f: f, lambda f: f]

@decorators[0]
def f():
    pass

@decorators[1]
def g():
    pass
