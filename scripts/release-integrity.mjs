import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const required = [
  "CHANGELOG.md",
  "CONTRIBUTING.md",
  "README.md",
  "SECURITY.md",
  "assets/banner.png",
  "docs/arquitectura.md",
  "docs/integracion-cli.md",
  "docs/modelo-economico.md",
  "docs/operacion.md",
  "docs/seguridad-y-riesgo.md",
];

for (const relative of required) {
  try {
    readFileSync(resolve(root, relative));
  } catch {
    console.error(`Missing release artifact: ${relative}`);
    process.exit(1);
  }
}

const packageJson = JSON.parse(readFileSync(resolve(root, "package.json"), "utf8"));
const lock = JSON.parse(readFileSync(resolve(root, "package-lock.json"), "utf8"));
if (packageJson.version !== "1.0.0" || lock.packages?.[""]?.version !== "1.0.0") {
  console.error("Package metadata must converge on version 1.0.0.");
  process.exit(1);
}

const readme = readFileSync(resolve(root, "README.md"), "utf8");
const changelog = readFileSync(resolve(root, "CHANGELOG.md"), "utf8");
if (!readme.includes("![Banner de MonolithDTL](./assets/banner.png)")) {
  console.error("README banner contract is missing.");
  process.exit(1);
}
if (!changelog.includes("## [1.0.0]")) {
  console.error("CHANGELOG does not describe version 1.0.0.");
  process.exit(1);
}

const png = readFileSync(resolve(root, "assets/banner.png"));
if (png.toString("hex", 0, 8) !== "89504e470d0a1a0a") {
  console.error("Banner is not a valid PNG.");
  process.exit(1);
}
const width = png.readUInt32BE(16);
const height = png.readUInt32BE(20);
const ratio = width / height;
if (width < 1600 || height < 900 || ratio < 1.7 || ratio > 1.85) {
  console.error(`Banner dimensions are outside the release contract: ${width}x${height}.`);
  process.exit(1);
}

const tracked = spawnSync("git", ["ls-files", "build", "dist", "coverage"], {
  cwd: root,
  encoding: "utf8",
  shell: false,
});
if (tracked.status !== 0 || tracked.stdout.trim() !== "") {
  console.error("Generated artifacts must not be versioned.");
  process.exit(1);
}
console.log(`Release integrity verified: ${packageJson.version}, banner ${width}x${height}.`);
