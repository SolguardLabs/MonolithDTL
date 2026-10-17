import test from "node:test";
import assert from "node:assert/strict";
import { assertCommon, byId, events, runScenario } from "../helpers/monolith.ts";

test("escrow settlement can release and refund under one local obligation", () => {
  const payload = runScenario("escrow-settlement");
  assertCommon(payload, "escrow-settlement");

  const escrow = byId(payload.escrows, "escrow-market-001");
  assert.equal(escrow.state, "refunded");
  assert.equal(escrow.released, 180_000_000);
  assert.equal(escrow.refunded, 120_000_000);
  assert.equal(payload.invariants.obligations_reconciled, true);
  assert.equal(events(payload, "escrow_released").length, 1);
  assert.equal(events(payload, "escrow_refunded").length, 1);
});

test("route operator flow settles all legs and records operator utilization", () => {
  const payload = runScenario("route-operator-flow");
  assertCommon(payload, "route-operator-flow");

  const route = byId(payload.routes, "route-operator-001");
  assert.equal(route.status, "settled");
  assert.equal(route.legs.length, 2);
  assert.equal(route.settled_amount, 200_000_000);
  assert.ok(
    payload.operators.some((operator) => operator.id === "op-alpha" && operator.used_today > 0),
  );
  assert.equal(payload.invariants.routes_conserved, true);
});

test("capital stress exposes deterministic charges and a monotonic ladder", () => {
  const payload = runScenario("capital-stress");
  assertCommon(payload, "capital-stress");

  assert.ok(payload.capital.charges.length >= 4);
  assert.ok(payload.capital.concentration_hhi_ppm > 0);
  assert.ok(payload.capital.liquidity_coverage_bps > 0);
  assert.equal(payload.capital.stress_ladder[0].name, "base");
  assert.equal(payload.capital.stress_ladder.at(-1)?.name, "reverse");
  assert.ok(
    payload.capital.stress_ladder.at(-1)!.required_capital >
      payload.capital.stress_ladder[0].required_capital,
  );
});
