# Python 3.10+
# f-string constant merging and escape sequence handling
x = f"hello {name!r} world"
msg = (
    f"value {x:.4f} > "
    f"expected {y:.4f}"
)
joined = (
    f"prefix {a} "
    "middle "
    f"{b} suffix"
)
esc = f"\x1b[{cod}m"
