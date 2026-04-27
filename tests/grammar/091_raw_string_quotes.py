_DECL = re.compile(r'(\'[^\']*\'|"[^"]*")\s*').match
_QUOTED = re.compile(r"^\s*=\s*\"([^\"\\]*(?:\\.[^\"\\]*)*)\"")
ESCAPE = r"\\"
DOUBLE_BACK = r'\\'
TICK = r'\''
TICK2 = r"\""
PATH_BACK = r"\\Server\Share\\"
RAW_TRIPLE = r"""abc\d\efg"""
