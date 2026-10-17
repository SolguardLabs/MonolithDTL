import test from "node:test";
import assert from "node:assert/strict";
import { assertCommon, localDebt, runScenario } from "../helpers/monolith.ts";

test("vault redemption guard rejects when shared debt is still high", () => {
  const payload = runScenario("vault-redemption-guard");
  assertCommon(payload, "vault-redemption-guard");

  assert.equal(payload.redemptions.length, 1);
  assert.equal(payload.redemptions[0].status, "rejected");
  assert.equal(payload.redemptions[0].reason, "blocked_by_global_debt");
  assert.equal(localDebt(payload, "vault", "vault-usd-core", "alice"), 700_000_000);
  assert.equal(payload.invariants.obligations_reconciled, true);
});
