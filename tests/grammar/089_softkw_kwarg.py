def call(case=None, match=None, type=None):
    return (case, match, type)
def use():
    call(case=1, match=2, type=3)
    call(case=case, match=match)
    call(*args, case=x)
    call(case=case, **kw)
