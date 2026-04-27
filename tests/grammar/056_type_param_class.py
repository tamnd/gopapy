# Python 3.12+: class Box[T]:
class Box[T]:
    def __init__(self, x: T) -> None:
        self.x = x

class Pair[T, U](Base):
    pass

class NoBases[T, U]:
    pass
