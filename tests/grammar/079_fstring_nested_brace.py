# Nested string inside an f-string interpolation: the `{` inside the
# nested non-f-string is plain text and must not start an interpolation.
s = f'{"{"}'
t = f'{f"x={x}"}'
