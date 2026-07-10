import test from "node:test";
import assert from "node:assert/strict";
import { assertCommon, byId, localDebt, runScenario } from "../helpers/monolith.ts";

test("vault redemption guard rejects when shared debt is still high", () => {
  const payload = runScenario("vault-redemption-guard");
  assertCommon(payload, "vault-redemption-guard");

  assert.equal(payload.redemptions.length, 1);
  assert.equal(payload.redemptions[0].status, "rejected");
  assert.equal(payload.redemptions[0].reason, "blocked_by_global_debt");
  assert.equal(localDebt(payload, "vault", "vault-usd-core", "alice"), 700_000_000);
  assert.equal(payload.invariants.global_debt_matches_local, true);
});

test("cross-product redemption exposes phantom debt drift", () => {
  const payload = runScenario("cross-product-redemption");
  assertCommon(payload, "cross-product-redemption");

  const escrow = byId(payload.escrows, "escrow-cross-001");
  assert.equal(escrow.state, "refunded");

  const redemption = payload.redemptions[0];
  assert.equal(redemption.status, "processed");
  assert.equal(redemption.blocked_by_debt, 0);
  assert.ok(redemption.expected_out > 0);

  const vaultDebt = localDebt(payload, "vault", "vault-usd-core", "alice");
  assert.equal(vaultDebt, 900_000_000);
  assert.equal(payload.invariants.global_debt_matches_local, false);
  assert.equal(payload.invariants.vault_redeems_local_debt, false);
  assert.equal(payload.risk.cross_product_drift, true);
  assert.equal(payload.risk.phantom_debt, 900_000_000);
  assert.ok(payload.reconciliation.some((line) => line.account === "alice" && line.severity === "critical"));
});
