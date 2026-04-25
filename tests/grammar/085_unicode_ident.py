# PEP 3131 / UAX #31: identifiers may contain non-ASCII letters and the
# Other_ID_Continue tag-character block (U+E0100..U+E01EF). Without the
# extra range, lexing `x` followed by a tag character used to spin
# catastrophically while trying every operator alternative.
class T:
    ä = 1
    蟒 = 3
    x󠄀 = 4
