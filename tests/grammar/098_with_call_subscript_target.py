def f(ctx, d):
    with ctx() as d[0]:
        pass
    with ctx() as d[0][1]:
        pass
    with ctx() as d['key']:
        pass
