import { createInterface } from "node:readline";

for await (const line of createInterface({ input: process.stdin })) {
  console.log(line);
}
