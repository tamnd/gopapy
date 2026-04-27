# Python 3.12+: type parameter syntax (PEP 695)
type Vector = list[float]
type Matrix[T] = list[list[T]]

def first[T](lst: list[T]) -> T:
    return lst[0]

class Stack[T]:
    def push(self, item: T) -> None: ...
    def pop(self) -> T: ...

def zip_two[T, S](a: list[T], b: list[S]) -> list[tuple[T, S]]:
    return list(zip(a, b))
