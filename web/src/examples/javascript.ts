import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const javascript: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
console.log("Hello, world!");
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
const readline = require("node:readline");

const rl = readline.createInterface({ input: process.stdin });
rl.on("line", (line) => {
  const name = line.trim();
  if (name) console.log(\`Hello, \${name}!\`);
});
`,
    stdin: NAMES,
  },
  {
    id: 'fibonacci',
    title: 'Fibonacci',
    code: source`
function* fibonacci() {
  let [a, b] = [0, 1];
  while (true) {
    yield a;
    [a, b] = [b, a + b];
  }
}

const numbers = [];
for (const n of fibonacci()) {
  if (numbers.length === 20) break;
  numbers.push(n);
}
console.log(numbers.join(" "));
`,
  },
  {
    id: 'showcase',
    title: 'Async/await & Promise.all',
    code: source`
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function loadUser(id, delay) {
  await sleep(delay);
  if (id === 3) throw new Error(\`user \${id} not found\`);
  console.log(\`loaded user \${id}\`);
  return { id, name: \`user-\${id}\` };
}

async function main() {
  // Both requests run concurrently, but results keep their input order.
  const users = await Promise.all([loadUser(1, 60), loadUser(2, 20)]);
  console.log(users.map(({ name }) => name));

  const results = await Promise.allSettled([loadUser(4, 10), loadUser(3, 30)]);
  for (const result of results) {
    if (result.status === "fulfilled") {
      console.log("ok:", result.value.name);
    } else {
      console.log("failed:", result.reason.message);
    }
  }
}

main();
`,
  },
];
