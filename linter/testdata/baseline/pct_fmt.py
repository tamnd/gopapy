# Fixture for F501 (% format mismatch).

x = "%s %s" % (1,)
y = "%s" % (1, 2)
ok_one = "%s" % "a"
ok_two = "%s %d" % ("a", 1)
escaped = "100%%" % ()
not_format = 7 % 3
