import { readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const src = join(root, "src");
let lines = 0;

for (const file of readdirSync(src).sort()) {
  if (!file.endsWith(".go")) continue;
  const text = readFileSync(join(src, file), "utf8");
  lines += text.split(/\r?\n/).filter((line) => line.trim().length > 0).length;
}

console.log(lines);

