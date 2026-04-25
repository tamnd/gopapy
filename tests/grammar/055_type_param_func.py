def first[T](xs: list[T]) -> T:
    return xs[0]

def pair[T, U](a: T, b: U) -> tuple[T, U]:
    return (a, b)

def identity[T](x: T) -> T:
    return x
