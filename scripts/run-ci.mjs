import { readdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const prettier = join(root, "node_modules", "prettier", "bin", "prettier.cjs");

function run(label, command, args) {
  console.log(`\n[ci] ${label}`);
  const result = spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    stdio: "inherit",
    shell: false,
  });
  if (result.status !== 0) process.exit(result.status ?? 1);
}

function formattedFiles(directory = root) {
  const ignored = new Set([".git", "build", "coverage", "dist", "node_modules"]);
  const extensions = new Set([".json", ".md", ".mjs", ".ts", ".yaml", ".yml"]);
  const files = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (ignored.has(entry.name)) continue;
    const path = join(directory, entry.name);
    if (entry.isDirectory()) files.push(...formattedFiles(path));
    else if ([...extensions].some((extension) => entry.name.endsWith(extension))) files.push(path);
  }
  return files.sort();
}

const gofmt = spawnSync("gofmt", ["-l", "src"], { cwd: root, encoding: "utf8", shell: false });
const preservedSources = new Set([
  "src/debt_engine.go",
  "src/ledger.go",
  "src/escrow.go",
  "src/vault.go",
]);
const unformatted = gofmt.stdout
  .split(/\r?\n/)
  .map((file) => file.replaceAll("\\", "/"))
  .filter((file) => file && !preservedSources.has(file));
if (gofmt.status !== 0 || unformatted.length > 0) {
  console.error(gofmt.stderr || `Go files require formatting:\n${unformatted.join("\n")}`);
  process.exit(gofmt.status || 1);
}

run("go vet", "go", ["vet", "./..."]);
run("go tests", "go", ["test", "./...", "-count=1"]);
run("production build", process.execPath, ["scripts/build.mjs", "--clean"]);
run("integration tests", process.execPath, ["scripts/run-tests.mjs"]);
run("prettier", process.execPath, [prettier, "--check", ...formattedFiles()]);
run("public-surface scan", process.execPath, ["scripts/scan-public-surface.mjs"]);
run("release integrity", process.execPath, ["scripts/release-integrity.mjs"]);
run("line count", process.execPath, ["scripts/check-loc.mjs"]);

rmSync(join(root, "build"), { recursive: true, force: true });
console.log("\n[ci] all checks passed");
