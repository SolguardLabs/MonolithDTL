import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const root = resolve(fileURLToPath(new URL("../../", import.meta.url)));
const exeName = process.platform === "win32" ? "monolithdtl.exe" : "monolithdtl";
const defaultBin = join(root, "build", exeName);

export type ScenarioPayload = {
  lab: string;
  scenario: string;
  network_id: string;
  clock: number;
  state_digest: string;
  assets: Array<Record<string, any>>;
  accounts: Array<Record<string, any>>;
  operators: Array<Record<string, any>>;
  escrows: Array<Record<string, any>>;
  vaults: Array<Record<string, any>>;
  vault_positions: Array<Record<string, any>>;
  routes: Array<Record<string, any>>;
  redemptions: Array<Record<string, any>>;
  global_debt: Array<Record<string, any>>;
  local_debt: Array<Record<string, any>>;
  reconciliation: Array<Record<string, any>>;
  events: Array<Record<string, any>>;
  ledger_entries: Array<Record<string, any>>;
  invariants: Record<string, any>;
  risk: Record<string, any>;
  notes: string[];
};

export function binaryPath(): string {
  return process.env.MONOLITH_BIN ?? defaultBin;
}

export function ensureBuilt(): void {
  if (process.env.MONOLITH_BIN) return;
  if (existsSync(defaultBin)) return;
  const result = spawnSync(process.execPath, ["scripts/build.mjs"], {
    cwd: root,
    encoding: "utf8",
    stdio: "pipe",
  });
  if (result.status !== 0) {
    throw new Error(["build failed", result.stdout.trim(), result.stderr.trim()].filter(Boolean).join("\n"));
  }
}

export function runBinary(args: string[], expectSuccess = true) {
  ensureBuilt();
  const result = spawnSync(binaryPath(), args, {
    cwd: root,
    encoding: "utf8",
    stdio: "pipe",
  });
  if (expectSuccess && result.status !== 0) {
    throw new Error(
      [`monolithdtl ${args.join(" ")} failed`, result.stdout.trim(), result.stderr.trim()]
        .filter(Boolean)
        .join("\n"),
    );
  }
  return result;
}

export function listScenarios(): string[] {
  return runBinary(["--list"]).stdout.trim().split(/\r?\n/).filter(Boolean);
}

export function runScenario(name: string): ScenarioPayload {
  const result = runBinary(["scenario", name]);
  return JSON.parse(result.stdout) as ScenarioPayload;
}

export function byId<T extends Record<string, any>>(items: T[], id: string): T {
  const found = items.find((item) => item.id === id);
  assert.ok(found, `missing id ${id}`);
  return found;
}

export function events(payload: ScenarioPayload, kind: string): Array<Record<string, any>> {
  return payload.events.filter((event) => event.kind === kind);
}

export function accountBalance(payload: ScenarioPayload, account: string, asset: string): number {
  const row = payload.accounts.find((item) => item.account === account && item.asset === asset);
  assert.ok(row, `missing balance ${account} ${asset}`);
  return row.amount;
}

export function localDebt(payload: ScenarioPayload, product: string, productId: string, account: string): number {
  const row = payload.local_debt.find(
    (item) => item.product === product && item.product_id === productId && item.account === account,
  );
  assert.ok(row, `missing local debt ${product} ${productId} ${account}`);
  return row.outstanding;
}

export function assertCommon(payload: ScenarioPayload, scenario: string): void {
  assert.equal(payload.lab, "MonolithDTL");
  assert.equal(payload.scenario, scenario);
  assert.equal(payload.network_id, "monolith-local-settlement");
  assert.match(payload.state_digest, /^[0-9a-f]{32}$/);
  assert.ok(payload.clock > 0);
  assert.ok(payload.assets.length >= 4);
  assert.ok(payload.accounts.length >= 8);
  assert.ok(payload.operators.length >= 2);
  assert.ok(payload.vaults.length >= 1);
  assert.ok(Array.isArray(payload.events));
  assert.equal(payload.invariants.ledger_non_negative, true);
  assert.equal(payload.invariants.local_solvency_non_negative, true);
  assert.equal(payload.invariants.operators_within_limit, true);
  assert.equal(payload.invariants.routes_conserved, true);
  assert.equal(payload.invariants.products_have_local_state, true);
}

