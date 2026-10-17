import { readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const testDir = join(root, "tests", "node");
const files = readdirSync(testDir)
  .filter((file) => file.endsWith(".test.ts"))
  .sort()
  .map((file) => join(testDir, file));

if (files.length === 0) {
  console.error("No integration tests found.");
  process.exit(1);
}

const result = spawnSync(process.execPath, ["--test", ...files], {
  cwd: root,
  encoding: "utf8",
  stdio: "inherit",
  shell: false,
});

process.exit(result.status ?? 1);
