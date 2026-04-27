# parser: new; Python 3.12+: PEP 701 f-string inline comments
a = f"""{
    1 + 2  # inline comment
}"""
b = f"""{
    x  # note
}"""
c = f"""{x = # self-doc with comment
}"""
