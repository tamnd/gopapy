# Three codepoints became printable (So) in Unicode 15.1 (Python 3.13): U+2FFC,
# U+2FFF, U+31EF. Five more became printable in Unicode 16.0 (Python 3.14):
# U+1B4F, U+1B7F, U+1C89, U+2427, U+31E4. Go ships Unicode 15.0 (all Cn),
# so pyRepr must gate each group on the target Python minor version.
x = "᭏᭿Ᲊ␧⿼⿿㇤㇯"
