# parser: new; Python 3.12+: PEP 701 f-string self-doc with inline comments
a = f"""{1 + 2 = # my comment
}"""
b = f"""{x = # note
  }"""
c = f"""{ # leading comment
  x = }"""
