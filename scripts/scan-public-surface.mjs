import { readFileSync } from "node:fs";
import { extname, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const tracked = spawnSync("git", ["ls-files", "-z"], { cwd: root, encoding: "utf8", shell: false });
if (tracked.status !== 0) {
  console.error(tracked.stderr);
  process.exit(tracked.status ?? 1);
}

const textExtensions = new Set(["", ".go", ".json", ".md", ".mjs", ".sh", ".ts", ".yaml", ".yml"]);
const forbidden = [
  "c" + "tf",
  "labo" + "ratorio",
  "cross-product-" + "redemption",
  "phantom" + " debt",
  "phantom_" + "debt",
  "deuda " + "fantasma",
  "vulnerable " + "por diseno",
  "vulnerable " + "por diseño",
  "vulnerabilidad " + "principal",
];
const secretPatterns = [
  /-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/,
  /\bghp_[A-Za-z0-9]{30,}\b/,
  /\bgithub_pat_[A-Za-z0-9_]{30,}\b/,
];
const findings = [];

for (const relative of tracked.stdout.split("\0").filter(Boolean)) {
  if (!textExtensions.has(extname(relative).toLowerCase())) continue;
  const text = readFileSync(resolve(root, relative), "utf8");
  const lower = text.toLowerCase();
  for (const phrase of forbidden) {
    if (lower.includes(phrase)) findings.push(`${relative}: forbidden public phrase`);
  }
  for (const pattern of secretPatterns) {
    if (pattern.test(text)) findings.push(`${relative}: credential-like material`);
  }
}

if (findings.length > 0) {
  console.error(findings.join("\n"));
  process.exit(1);
}
console.log(
  `Public surface clean: ${tracked.stdout.split("\0").filter(Boolean).length} tracked files.`,
);
