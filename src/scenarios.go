package main

import "fmt"

var scenarioNames = []string{
	"baseline",
	"escrow-settlement",
	"vault-redemption-guard",
	"route-operator-flow",
	"cross-product-redemption",
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
	case "cross-product-redemption":
		return scenarioCrossProductRedemption(), nil
	default:
		return ScenarioReport{}, fmt.Errorf("unknown scenario %s", name)
	}
}

func scenarioBaseline() ScenarioReport {
	ctx := NewLabContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 250_000_000, "op-alpha", "alice-borrow-001"))
	must(ctx.Vaults.Repay("vault-usd-core", "alice", 250_000_000, "alice-repay-001"))
	mustEscrow(ctx.Escrows.Create("escrow-baseline-001", "bob", "dave", "mUSD", 90_000_000, 2_000_000, "op-beta", 30))
	must(ctx.Escrows.Release("escrow-baseline-001", 90_000_000, "escrow-baseline-release"))
	must(ctx.Escrows.CaptureFee("escrow-baseline-001", "treasury", "escrow-baseline-fee"))
	return BuildReport(ctx, "baseline")
}

func scenarioEscrowSettlement() ScenarioReport {
	ctx := NewLabContext()
	runCommonBootstrap(ctx)
	mustEscrow(ctx.Escrows.Create("escrow-market-001", "alice", "bob", "mUSD", 300_000_000, 4_500_000, "op-alpha", 40))
	must(ctx.Escrows.Release("escrow-market-001", 180_000_000, "escrow-market-release-001"))
	must(ctx.Escrows.SettleRefund("escrow-market-001", 120_000_000, "escrow-market-refund-001"))
	must(ctx.Escrows.CaptureFee("escrow-market-001", "treasury", "escrow-market-fee"))
	return BuildReport(ctx, "escrow-settlement")
}

func scenarioVaultRedemptionGuard() ScenarioReport {
	ctx := NewLabContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 700_000_000, "op-alpha", "guard-borrow-001"))
	ctx.Redemptions.RequestVault("vault-usd-core", "alice", 500_000_000, 250_000_000, "guard-redeem-001")
	return BuildReport(ctx, "vault-redemption-guard")
}

func scenarioRouteOperatorFlow() ScenarioReport {
	ctx := NewLabContext()
	runCommonBootstrap(ctx)
	mustEscrow(ctx.Escrows.Create("escrow-route-001", "dave", "bob", "mUSD", 110_000_000, 2_000_000, "op-beta", 80))
	mustRoute(ctx.Routes.Create("route-operator-001", "dave", "bob", "mUSD", 200_000_000, 195_000_000, 90, "op-alpha"))
	must(ctx.Routes.AddLeg("route-operator-001", "dave", "reserve-route", "mUSD", 100_000_000, ProductRoute, "route-operator-001"))
	must(ctx.Routes.AddLeg("route-operator-001", "reserve-route", "bob", "mUSD", 100_000_000, ProductEscrow, "escrow-route-001"))
	must(ctx.Routes.Settle("route-operator-001", "route-operator-settle"))
	must(ctx.Escrows.Release("escrow-route-001", 110_000_000, "escrow-route-release"))
	return BuildReport(ctx, "route-operator-flow")
}

func scenarioCrossProductRedemption() ScenarioReport {
	ctx := NewLabContext()
	runCommonBootstrap(ctx)
	must(ctx.Vaults.Deposit("vault-usd-core", "alice", 400_000_000, "alice-cross-extra-deposit"))
	must(ctx.Vaults.Borrow("vault-usd-core", "alice", 900_000_000, "op-alpha", "cross-vault-borrow"))
	mustEscrow(ctx.Escrows.Create("escrow-cross-001", "alice", "bob", "mUSD", 650_000_000, 900_000_000, "op-alpha", 50))
	must(ctx.Escrows.SettleRefund("escrow-cross-001", 650_000_000, "cross-escrow-refund"))
	ctx.Redemptions.RequestVault("vault-usd-core", "alice", 420_000_000, 150_000_000, "cross-vault-redeem")
	return BuildReport(ctx, "cross-product-redemption")
}
