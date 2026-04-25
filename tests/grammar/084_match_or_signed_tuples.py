# Match or-pattern with deep paren-tuples and signed-number literals;
# requires sufficient parser lookahead to discriminate.
match x:
    case ((a, b, c, d, e, f, g, h, i, 9) |
          (h, g, i, a, b, d, e, c, f, 10) |
          (g, b, a, c, d, -5, e, h, i, f) |
          (-1, d, f, b, g, e, i, a, h, c)):
        w = 0
