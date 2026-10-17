package main

type ProtocolContext struct {
	Ledger      *Ledger
	Debt        *DebtEngine
	Operators   *OperatorRegistry
	Escrows     *EscrowBook
	Vaults      *VaultBook
	Routes      *RouteBook
	Redemptions *RedemptionQueue
	Auditor     *Auditor
}

func NewProtocolContext() *ProtocolContext {
	ledger := NewLedger("monolith-local-settlement")
	seedAssets(ledger)
	seedAccounts(ledger)
	debt := NewDebtEngine(ledger)
	operators := NewOperatorRegistry(ledger)
	escrows := NewEscrowBook(ledger, debt, operators)
	vaults := NewVaultBook(ledger, debt, operators)
	routes := NewRouteBook(ledger, debt, operators)
	redemptions := NewRedemptionQueue(ledger, vaults)
	ctx := &ProtocolContext{
		Ledger:      ledger,
		Debt:        debt,
		Operators:   operators,
		Escrows:     escrows,
		Vaults:      vaults,
		Routes:      routes,
		Redemptions: redemptions,
	}
	ctx.Auditor = NewAuditor(ledger, debt, operators, escrows, vaults, routes, redemptions)
	seedOperators(ctx)
	seedVaults(ctx)
	return ctx
}

func seedAssets(ledger *Ledger) {
	ledger.AddAsset(Asset{ID: "mUSD", Symbol: "mUSD", Decimals: 6, RiskWeightBps: 10_000, HaircutBps: 0})
	ledger.AddAsset(Asset{ID: "mEUR", Symbol: "mEUR", Decimals: 6, RiskWeightBps: 9_800, HaircutBps: 50})
	ledger.AddAsset(Asset{ID: "rSOL", Symbol: "rSOL", Decimals: 9, RiskWeightBps: 8_500, HaircutBps: 750})
	ledger.AddAsset(Asset{ID: "vUSD", Symbol: "vUSD", Decimals: 6, RiskWeightBps: 10_000, HaircutBps: 0})
}

func seedAccounts(ledger *Ledger) {
	ledger.OpenAccount("alice", "Alice", RoleUser, "retail", "kyc-basic")
	ledger.OpenAccount("bob", "Bob", RoleUser, "merchant")
	ledger.OpenAccount("carol", "Carol", RoleUser, "liquidity")
	ledger.OpenAccount("dave", "Dave", RoleUser, "arb")
	ledger.OpenAccount("reserve-vault-usd", "Vault USD Reserve", RoleReserve, "vault")
	ledger.OpenAccount("reserve-escrow", "Escrow Reserve", RoleReserve, "escrow")
	ledger.OpenAccount("reserve-route", "Route Reserve", RoleReserve, "route")
	ledger.OpenAccount("treasury", "Treasury", RoleTreasury, "fees")
	ledger.OpenAccount("insurance", "Insurance", RoleInsurance, "backstop")
	ledger.OpenAccount("op-alpha-account", "Operator Alpha", RoleOperator, "operator")
	ledger.OpenAccount("op-beta-account", "Operator Beta", RoleOperator, "operator")
	_ = ledger.Credit(ProductSystem, "genesis", "alice", "mUSD", 3_200_000_000, "genesis", "genesis mint")
	_ = ledger.Credit(ProductSystem, "genesis", "bob", "mUSD", 1_100_000_000, "genesis", "genesis mint")
	_ = ledger.Credit(ProductSystem, "genesis", "carol", "mUSD", 2_400_000_000, "genesis", "genesis mint")
	_ = ledger.Credit(ProductSystem, "genesis", "dave", "mUSD", 1_500_000_000, "genesis", "genesis mint")
	_ = ledger.Credit(ProductSystem, "genesis", "reserve-vault-usd", "mUSD", 4_000_000_000, "genesis", "genesis reserve")
	_ = ledger.Credit(ProductSystem, "genesis", "reserve-escrow", "mUSD", 500_000_000, "genesis", "genesis reserve")
	_ = ledger.Credit(ProductSystem, "genesis", "reserve-route", "mUSD", 900_000_000, "genesis", "genesis reserve")
	_ = ledger.Credit(ProductSystem, "genesis", "treasury", "mUSD", 250_000_000, "genesis", "genesis reserve")
	_ = ledger.Credit(ProductSystem, "genesis", "insurance", "mUSD", 750_000_000, "genesis", "genesis reserve")
}

func seedOperators(ctx *ProtocolContext) {
	_, _ = ctx.Operators.Register("op-alpha", "op-alpha-account", "Alpha Settlement", 6_000_000_000, "tier-1", ProductEscrow, ProductVault, ProductRoute)
	_, _ = ctx.Operators.Register("op-beta", "op-beta-account", "Beta Redemption", 3_000_000_000, "tier-2", ProductVault, ProductRedemption, ProductRoute)
	_ = ctx.Operators.Authorize("op-alpha", ProductRedemption, 8_000)
	_ = ctx.Operators.Authorize("op-beta", ProductEscrow, 4_000)
}

func seedVaults(ctx *ProtocolContext) {
	_, _ = ctx.Vaults.Create("vault-usd-core", "mUSD", "reserve-vault-usd", "vUSD", 30)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func mustEscrow(_ *Escrow, err error) {
	must(err)
}
func mustRoute(_ *RoutePlan, err error) {
	must(err)
}

func runCommonBootstrap(ctx *ProtocolContext) {
	must(ctx.Vaults.Deposit("vault-usd-core", "alice", 1_000_000_000, "alice-deposit-001"))
	must(ctx.Vaults.Deposit("vault-usd-core", "carol", 1_400_000_000, "carol-deposit-001"))
	mustRoute(ctx.Routes.Create("route-bootstrap-001", "bob", "carol", "mUSD", 120_000_000, 118_000_000, 60, "op-alpha"))
	must(ctx.Routes.AddLeg("route-bootstrap-001", "bob", "reserve-route", "mUSD", 60_000_000, ProductRoute, "route-bootstrap-001"))
	must(ctx.Routes.AddLeg("route-bootstrap-001", "reserve-route", "carol", "mUSD", 60_000_000, ProductRoute, "route-bootstrap-001"))
	must(ctx.Routes.Settle("route-bootstrap-001", "route-bootstrap-settle"))
}
