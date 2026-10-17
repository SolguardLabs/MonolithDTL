import { readFileSync } from "node:fs";
import { extname, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const tracked = spawnSync("git", ["ls-files", "-z"], { cwd: root, encoding: "utf8", shell: false });
if (tracked.status !== 0) process.exit(tracked.status ?? 1);

const textExtensions = new Set(["", ".go", ".json", ".md", ".mjs", ".sh", ".ts", ".yaml", ".yml"]);
let files = 0;
let nonblank = 0;
const byExtension = {};

for (const relative of tracked.stdout.split("\0").filter(Boolean)) {
  const extension = extname(relative).toLowerCase();
  if (!textExtensions.has(extension)) continue;
  const lines = readFileSync(resolve(root, relative), "utf8")
    .split(/\r?\n/)
    .filter((line) => line.trim().length > 0).length;
  files += 1;
  nonblank += lines;
  const key = extension || "text";
  byExtension[key] = (byExtension[key] ?? 0) + lines;
}

console.log(JSON.stringify({ files, nonblank, by_extension: byExtension }));
