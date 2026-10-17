package main

import (
	"fmt"
	"sort"
)

type Auditor struct {
	ledger      *Ledger
	debt        *DebtEngine
	operators   *OperatorRegistry
	escrows     *EscrowBook
	vaults      *VaultBook
	routes      *RouteBook
	redemptions *RedemptionQueue
}

func NewAuditor(ledger *Ledger, debt *DebtEngine, operators *OperatorRegistry, escrows *EscrowBook, vaults *VaultBook, routes *RouteBook, redemptions *RedemptionQueue) *Auditor {
	return &Auditor{
		ledger:      ledger,
		debt:        debt,
		operators:   operators,
		escrows:     escrows,
		vaults:      vaults,
		routes:      routes,
		redemptions: redemptions,
	}
}

func (a *Auditor) Invariants() InvariantReport {
	reds := []RedemptionRequest{}
	if a.redemptions != nil {
		reds = a.redemptions.Redemptions()
	}
	return InvariantReport{
		LedgerNonNegative:        a.ledger.LedgerNonNegative(),
		LocalSolvencyNonNegative: a.debt.LocalSolvencyNonNegative(),
		OperatorsWithinLimit:     a.operators.OperatorsWithinLimit(),
		RoutesConserved:          a.routes.RoutesConserved(),
		ProductsHaveLocalState:   a.debt.ProductsHaveLocalState(),
		ObligationsReconciled:    !a.debt.HasCrossProductDrift(),
		RedemptionsWithinPolicy:  a.vaults.RedeemsRespectLocalDebt(reds),
	}
}

func (a *Auditor) Risk() RiskReport {
	local := a.debt.TotalLocalOutstanding()
	global := a.ledger.TotalGlobalDebt()
	processed := 0
	rejected := 0
	if a.redemptions != nil {
		processed = a.redemptions.ProcessedCount()
		rejected = a.redemptions.RejectedCount()
	}
	return RiskReport{
		ExecutedEscrows:        a.escrows.ExecutedCount(),
		ExecutedRoutes:         a.routes.ExecutedCount(),
		ProcessedRedemptions:   processed,
		RejectedRedemptions:    rejected,
		GlobalDebt:             global,
		LocalDebt:              local,
		ReconciliationVariance: local - global,
		ReconciliationAlert:    a.debt.HasCrossProductDrift(),
		UnmatchedObligations:   a.debt.PhantomDebt(),
		ReserveVariance:        a.vaults.ReserveLeak(),
	}
}

func (a *Auditor) Events() []Event {
	events := make([]Event, 0)
	events = append(events, a.ledger.Events()...)
	events = append(events, a.debt.Events()...)
	sort.Slice(events, func(i int, j int) bool {
		if events[i].Clock == events[j].Clock {
			return events[i].ID < events[j].ID
		}
		return events[i].Clock < events[j].Clock
	})
	return events
}

func (a *Auditor) Notes(scenario string) []string {
	inv := a.Invariants()
	risk := a.Risk()
	notes := []string{
		"scenario=" + scenario,
		fmt.Sprintf("local_debt=%d", risk.LocalDebt),
		fmt.Sprintf("global_debt=%d", risk.GlobalDebt),
	}
	if !inv.ObligationsReconciled {
		notes = append(notes, "obligation reconciliation requires operator review")
	}
	if !inv.RedemptionsWithinPolicy {
		notes = append(notes, "redemption policy exception requires operator review")
	}
	if risk.ReconciliationAlert {
		notes = append(notes, "reconciliation alert is active")
	}
	return notes
}

func (a *Auditor) Digest(scenario string) string {
	parts := []string{
		"protocol:MonolithDTL",
		"scenario:" + scenario,
		"ledger:" + a.ledger.SnapshotDigest(),
	}
	for _, line := range a.debt.LocalViews() {
		parts = append(parts, fmt.Sprintf("local:%s:%d:%s", line.Key, line.Outstanding, line.State))
	}
	for _, route := range a.routes.Routes() {
		parts = append(parts, fmt.Sprintf("route:%s:%s:%d", route.ID, route.Status, route.SettledAmount))
	}
	for _, redemption := range a.redemptions.Redemptions() {
		parts = append(parts, fmt.Sprintf("red:%s:%s:%d", redemption.ID, redemption.Status, redemption.ExpectedOut))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func (a *Auditor) CriticalLines() []ReconciliationLine {
	out := make([]ReconciliationLine, 0)
	for _, line := range a.debt.Reconcile() {
		if line.Severity == "critical" {
			out = append(out, line)
		}
	}
	return out
}

func (a *Auditor) ProductSummary() map[string]Amount {
	return map[string]Amount{
		"escrow": a.debt.OutstandingByProduct(ProductEscrow),
		"vault":  a.debt.OutstandingByProduct(ProductVault),
		"route":  a.debt.OutstandingByProduct(ProductRoute),
	}
}
