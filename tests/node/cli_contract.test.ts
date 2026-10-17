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
    "capital-stress",
  ]);
});

test("baseline emits stable report contract", () => {
  const payload = runScenario("baseline");
  assertCommon(payload, "baseline");
  assert.equal(payload.invariants.obligations_reconciled, true);
  assert.equal(payload.invariants.redemptions_within_policy, true);
  assert.equal(payload.risk.reconciliation_alert, false);
  assert.equal(payload.capital.decision, "open");
});

test("unknown scenario exits with failure", () => {
  const result = runBinary(["scenario", "missing-scenario"], false);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /unknown scenario/);
});
