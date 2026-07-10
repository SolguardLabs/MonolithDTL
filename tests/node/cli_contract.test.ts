import test from "node:test";
import assert from "node:assert/strict";
import { assertCommon, listScenarios, runBinary, runScenario } from "../helpers/monolith.ts";

test("cli lists deterministic public scenarios", () => {
  const scenarios = listScenarios();
  assert.deepEqual(scenarios, [
    "baseline",
    "escrow-settlement",
    "vault-redemption-guard",
    "route-operator-flow",
    "cross-product-redemption",
  ]);
});

test("baseline emits stable report contract", () => {
  const payload = runScenario("baseline");
  assertCommon(payload, "baseline");
  assert.equal(payload.invariants.global_debt_matches_local, true);
  assert.equal(payload.invariants.vault_redeems_local_debt, true);
  assert.equal(payload.risk.cross_product_drift, false);
});

test("unknown scenario exits with failure", () => {
  const result = runBinary(["scenario", "missing-scenario"], false);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /unknown scenario/);
});

