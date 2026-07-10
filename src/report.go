package main

import (
	"encoding/json"
	"fmt"
	"sort"
)

func BuildReport(ctx *LabContext, scenario string) ScenarioReport {
	redemptions := ctx.Redemptions.Redemptions()
	report := ScenarioReport{
		Lab:            "MonolithDTL",
		Scenario:       scenario,
		NetworkID:      "monolith-local-settlement",
		Clock:          ctx.Ledger.Clock(),
		Assets:         ctx.Ledger.Assets(),
		Accounts:       ctx.Ledger.BalanceViews(),
		Operators:      ctx.Operators.Views(),
		Escrows:        ctx.Escrows.Escrows(),
		Vaults:         ctx.Vaults.Vaults(),
		VaultPositions: ctx.Vaults.Positions(),
		Routes:         ctx.Routes.Routes(),
		Redemptions:    redemptions,
		GlobalDebt:     ctx.Ledger.DebtViews(),
		LocalDebt:      ctx.Debt.LocalViews(),
		Reconciliation: ctx.Debt.Reconcile(),
		Events:         ctx.Auditor.Events(),
		LedgerEntries:  ctx.Ledger.Entries(),
		Invariants:     ctx.Auditor.Invariants(),
		Risk:           ctx.Auditor.Risk(),
		Notes:          ctx.Auditor.Notes(scenario),
	}
	report.StateDigest = reportDigest(report)
	return report
}

func EncodeReport(report ScenarioReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func reportDigest(report ScenarioReport) string {
	parts := []string{
		"lab:" + report.Lab,
		"scenario:" + report.Scenario,
		"network:" + report.NetworkID,
		fmt.Sprintf("clock:%d", report.Clock),
	}
	for _, account := range report.Accounts {
		parts = append(parts, fmt.Sprintf("bal:%s:%s:%d", account.Account, account.Asset, account.Amount))
	}
	for _, debt := range report.GlobalDebt {
		parts = append(parts, fmt.Sprintf("gdebt:%s:%s:%d", debt.Account, debt.Asset, debt.Amount))
	}
	for _, local := range report.LocalDebt {
		parts = append(parts, fmt.Sprintf("ldebt:%s:%d:%s", local.Key, local.Outstanding, local.State))
	}
	for _, redemption := range report.Redemptions {
		parts = append(parts, fmt.Sprintf("red:%s:%s:%d:%d", redemption.ID, redemption.Status, redemption.ExpectedOut, redemption.BlockedByDebt))
	}
	for _, route := range report.Routes {
		parts = append(parts, fmt.Sprintf("route:%s:%s:%d", route.ID, route.Status, route.SettledAmount))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func PrintScenario(name string) error {
	report, err := RunScenario(name)
	if err != nil {
		return err
	}
	data, err := EncodeReport(report)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

