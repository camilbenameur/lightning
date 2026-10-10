import { writeFileSync } from "node:fs";

for (const path of ["/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"]) {
  try {
    writeFileSync(path, "");
    console.log(`${path}: ok`);
  } catch {
    console.log(`${path}: error`);
  }
}
