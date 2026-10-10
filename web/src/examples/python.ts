import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const python: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
print("Hello, world!")
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
import sys

for line in sys.stdin:
    if name := line.strip():
        print(f"Hello, {name}!")
`,
    stdin: NAMES,
  },
  {
    id: 'fizzbuzz',
    title: 'FizzBuzz',
    code: source`
def fizzbuzz(n: int) -> str:
    match n % 3, n % 5:
        case 0, 0:
            return "FizzBuzz"
        case 0, _:
            return "Fizz"
        case _, 0:
            return "Buzz"
        case _:
            return str(n)


for n in range(1, 21):
    print(fizzbuzz(n))
`,
  },
  {
    id: 'showcase',
    title: 'Dataclasses & comprehensions',
    code: source`
from collections import Counter
from dataclasses import dataclass


@dataclass(frozen=True, order=True)
class Book:
    title: str
    author: str
    pages: int

    @property
    def is_long(self) -> bool:
        return self.pages > 300


library = [
    Book("Dune", "Herbert", 412),
    Book("Emma", "Austen", 474),
    Book("Persuasion", "Austen", 249),
    Book("Neuromancer", "Gibson", 271),
]

print([book.title for book in library if book.is_long])
print({book.title: book.pages for book in sorted(library)})
print(Counter(book.author for book in library).most_common(1))
print(max(library, key=lambda book: book.pages))
`,
  },
];
