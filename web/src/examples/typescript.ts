import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const typescript: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
const message: string = "Hello, world!";
console.log(message);
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
import { createInterface } from "node:readline";

for await (const line of createInterface({ input: process.stdin })) {
  const name = line.trim();
  if (name) console.log(\`Hello, \${name}!\`);
}
`,
    stdin: NAMES,
  },
  {
    id: 'fizzbuzz',
    title: 'FizzBuzz',
    code: source`
type FizzBuzz = "Fizz" | "Buzz" | "FizzBuzz";

function fizzbuzz(n: number): FizzBuzz | number {
  if (n % 15 === 0) return "FizzBuzz";
  if (n % 3 === 0) return "Fizz";
  if (n % 5 === 0) return "Buzz";
  return n;
}

for (let i = 1; i <= 20; i++) {
  console.log(fizzbuzz(i));
}
`,
  },
  {
    id: 'showcase',
    title: 'Generics, unions & enums',
    code: source`
enum Status {
  Ok = "ok",
  Failed = "failed",
}

type Result<T> =
  { status: Status.Ok; value: T } | { status: Status.Failed; error: string };

function parse(input: string): Result<number> {
  const value = Number(input);
  return Number.isNaN(value)
    ? { status: Status.Failed, error: \`"\${input}" is not a number\` }
    : { status: Status.Ok, value };
}

function mapResult<T, U>(result: Result<T>, fn: (value: T) => U): Result<U> {
  return result.status === Status.Ok
    ? { status: Status.Ok, value: fn(result.value) }
    : result;
}

for (const input of ["21", "4.5", "abc"]) {
  const doubled = mapResult(parse(input), (n) => n * 2);
  console.log(doubled.status === Status.Ok ? doubled.value : doubled.error);
}
`,
  },
];
