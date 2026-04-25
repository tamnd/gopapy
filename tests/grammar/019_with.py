with open("a") as f:
    f.read()
with open("a") as f, open("b") as g:
    pass
with (open("a") as f, open("b") as g):
    pass
