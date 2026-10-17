package main

import "fmt"

var scenarioNames = []string{
	"baseline",
	"escrow-settlement",
	"vault-redemption-guard",
	"route-operator-flow",
	"capital-stress",
}

func ListScenarios() []string {
	out := make([]string, len(scenarioNames))
	copy(out, scenarioNames)
	return out
}

func RunScenario(name string) (ScenarioReport, error) {
	switch name {
	case "baseline":
		return scenarioBaseline(), nil
	case "escrow-settlement":
		return scenarioEscrowSettlement(), nil
	case "vault-redemption-guard":
		return scenarioVaultRedemptionGuard(), nil
	case "route-operator-flow":
		return scenarioRouteOperatorFlow(), nil
	case "capital-stress":
		return scenarioCapitalStress(), nil
	default:
		return ScenarioReport{}, fmt.Errorf("unknown scenario %s", name)
	}
}

func scenarioBaseline() ScenarioReport {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 250_000_000, "op-alpha", "alice-borrow-001"))
	must(ctx.Vaults.Repay("vault-usd-core", "alice", 250_000_000, "alice-repay-001"))
	mustEscrow(ctx.Escrows.Create("escrow-baseline-001", "bob", "dave", "mUSD", 90_000_000, 2_000_000, "op-beta", 30))
	must(ctx.Escrows.Release("escrow-baseline-001", 90_000_000, "escrow-baseline-release"))
	must(ctx.Escrows.CaptureFee("escrow-baseline-001", "treasury", "escrow-baseline-fee"))
	return BuildReport(ctx, "baseline")
}

func scenarioEscrowSettlement() ScenarioReport {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	mustEscrow(ctx.Escrows.Create("escrow-market-001", "alice", "bob", "mUSD", 300_000_000, 4_500_000, "op-alpha", 40))
	must(ctx.Escrows.Release("escrow-market-001", 180_000_000, "escrow-market-release-001"))
	must(ctx.Escrows.SettleRefund("escrow-market-001", 120_000_000, "escrow-market-refund-001"))
	must(ctx.Escrows.CaptureFee("escrow-market-001", "treasury", "escrow-market-fee"))
	return BuildReport(ctx, "escrow-settlement")
}

func scenarioVaultRedemptionGuard() ScenarioReport {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 700_000_000, "op-alpha", "guard-borrow-001"))
	ctx.Redemptions.RequestVault("vault-usd-core", "alice", 500_000_000, 250_000_000, "guard-redeem-001")
	return BuildReport(ctx, "vault-redemption-guard")
}

func scenarioRouteOperatorFlow() ScenarioReport {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	mustEscrow(ctx.Escrows.Create("escrow-route-001", "dave", "bob", "mUSD", 110_000_000, 2_000_000, "op-beta", 80))
	mustRoute(ctx.Routes.Create("route-operator-001", "dave", "bob", "mUSD", 200_000_000, 195_000_000, 90, "op-alpha"))
	must(ctx.Routes.AddLeg("route-operator-001", "dave", "reserve-route", "mUSD", 100_000_000, ProductRoute, "route-operator-001"))
	must(ctx.Routes.AddLeg("route-operator-001", "reserve-route", "bob", "mUSD", 100_000_000, ProductEscrow, "escrow-route-001"))
	must(ctx.Routes.Settle("route-operator-001", "route-operator-settle"))
	must(ctx.Escrows.Release("escrow-route-001", 110_000_000, "escrow-route-release"))
	return BuildReport(ctx, "route-operator-flow")
}

func scenarioCapitalStress() ScenarioReport {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 650_000_000, "op-alpha", "stress-vault-borrow"))
	mustEscrow(ctx.Escrows.Create("escrow-stress-001", "dave", "bob", "mUSD", 420_000_000, 6_000_000, "op-beta", 20))
	mustRoute(ctx.Routes.Create("route-stress-001", "bob", "carol", "mUSD", 360_000_000, 350_000_000, 120, "op-alpha"))
	return BuildReport(ctx, "capital-stress")
}
