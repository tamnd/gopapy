# F541 fixtures: f-strings without any placeholder.

a = f"hello"          # F541
b = f""               # F541
c = f'no placeholder' # F541

# Negative cases — these must stay silent.
y = 1
d = f"value={y}"
e = f"{y!r}"
plain = "not an f-string"
