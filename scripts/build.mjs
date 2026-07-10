import { existsSync, mkdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const outDir = join(root, "build");
const exeName = process.platform === "win32" ? "monolithdtl.exe" : "monolithdtl";
const output = join(outDir, exeName);
const args = new Set(process.argv.slice(2));

function run(command, argv, options = {}) {
  return spawnSync(command, argv, {
    cwd: root,
    encoding: "utf8",
    stdio: options.stdio ?? "pipe",
    shell: false,
  });
}

function commandExists(command) {
  if (process.platform === "win32") {
    return run("where.exe", [command]).status === 0;
  }
  return spawnSync("sh", ["-c", `command -v "${command.replaceAll('"', '\\"')}"`], {
    cwd: root,
    encoding: "utf8",
    stdio: "pipe",
  }).status === 0;
}

if (args.has("--clean")) {
  rmSync(outDir, { recursive: true, force: true });
}

if (!commandExists("go")) {
  console.error("Go toolchain not found. Install Go 1.22+ or put go in PATH.");
  process.exit(1);
}

mkdirSync(outDir, { recursive: true });
const result = run("go", ["build", "-trimpath", "-o", output, "./src"], { stdio: "inherit" });
if (result.status !== 0) {
  process.exit(result.status ?? 1);
}
if (!existsSync(output)) {
  console.error(`Expected build artifact was not created: ${output}`);
  process.exit(1);
}
console.log(output);

