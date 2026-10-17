package main

import (
	"fmt"
	"sort"
)

type ProductPolicy struct {
	Product          ProductKind `json:"product"`
	Enabled          bool        `json:"enabled"`
	MinCoverageBps   int         `json:"min_coverage_bps"`
	MaxDriftBps      int         `json:"max_drift_bps"`
	RedemptionBuffer Amount      `json:"redemption_buffer"`
	OperatorCapBps   int         `json:"operator_cap_bps"`
	AllowNetting     bool        `json:"allow_netting"`
}

type RiskPolicy struct {
	Products        map[ProductKind]ProductPolicy
	GlobalDebtCap   Amount
	DustThreshold   Amount
	StrictReconcile bool
}

type AdmissionContext struct {
	Product     ProductKind
	ProductID   string
	Account     string
	Asset       string
	Amount      Amount
	LocalDebt   Amount
	GlobalDebt  Amount
	Collateral  Amount
	OperatorID  string
	RequestedBy string
	Operation   string
}

type AdmissionResult struct {
	Allowed        bool           `json:"allowed"`
	Reason         string         `json:"reason"`
	Product        ProductKind    `json:"product"`
	ProductID      string         `json:"product_id"`
	Account        string         `json:"account"`
	Asset          string         `json:"asset"`
	Amount         Amount         `json:"amount"`
	CoverageBps    int            `json:"coverage_bps"`
	DriftBps       int            `json:"drift_bps"`
	ObservedDebt   Amount         `json:"observed_debt"`
	RequiredDebt   Amount         `json:"required_debt"`
	PolicySnapshot map[string]int `json:"policy_snapshot"`
}

func DefaultRiskPolicy() RiskPolicy {
	return RiskPolicy{
		Products: map[ProductKind]ProductPolicy{
			ProductEscrow: {
				Product:        ProductEscrow,
				Enabled:        true,
				MinCoverageBps: 10_000,
				MaxDriftBps:    0,
				OperatorCapBps: 8_000,
				AllowNetting:   true,
			},
			ProductVault: {
				Product:          ProductVault,
				Enabled:          true,
				MinCoverageBps:   12_000,
				MaxDriftBps:      0,
				RedemptionBuffer: 1,
				OperatorCapBps:   7_000,
				AllowNetting:     false,
			},
			ProductRoute: {
				Product:        ProductRoute,
				Enabled:        true,
				MinCoverageBps: 10_000,
				MaxDriftBps:    100,
				OperatorCapBps: 9_000,
				AllowNetting:   true,
			},
			ProductRedemption: {
				Product:          ProductRedemption,
				Enabled:          true,
				MinCoverageBps:   10_000,
				MaxDriftBps:      0,
				RedemptionBuffer: 1,
				OperatorCapBps:   6_000,
				AllowNetting:     false,
			},
		},
		GlobalDebtCap:   9_000_000_000,
		DustThreshold:   10,
		StrictReconcile: false,
	}
}

func (p RiskPolicy) PolicyFor(product ProductKind) ProductPolicy {
	if item, ok := p.Products[product]; ok {
		return item
	}
	return ProductPolicy{
		Product:        product,
		Enabled:        false,
		MinCoverageBps: 10_000,
		MaxDriftBps:    0,
		OperatorCapBps: 0,
	}
}

func (p RiskPolicy) Admit(ctx AdmissionContext) AdmissionResult {
	policy := p.PolicyFor(ctx.Product)
	result := AdmissionResult{
		Product:   ctx.Product,
		ProductID: ctx.ProductID,
		Account:   ctx.Account,
		Asset:     ctx.Asset,
		Amount:    ctx.Amount,
		PolicySnapshot: map[string]int{
			"min_coverage_bps": policy.MinCoverageBps,
			"max_drift_bps":    policy.MaxDriftBps,
			"operator_cap_bps": policy.OperatorCapBps,
		},
	}
	result.ObservedDebt = ctx.GlobalDebt
	result.RequiredDebt = ctx.LocalDebt
	result.CoverageBps = RatioBps(ctx.Collateral, MaxAmountOf(ctx.LocalDebt+ctx.Amount, 1))
	result.DriftBps = RatioBps(AbsAmount(ctx.LocalDebt-ctx.GlobalDebt), MaxAmountOf(ctx.LocalDebt, 1))
	if !policy.Enabled {
		result.Allowed = false
		result.Reason = "product_disabled"
		return result
	}
	if ctx.Amount <= 0 {
		result.Allowed = false
		result.Reason = "amount_not_positive"
		return result
	}
	if p.GlobalDebtCap > 0 && ctx.GlobalDebt+ctx.Amount > p.GlobalDebtCap {
		result.Allowed = false
		result.Reason = "global_debt_cap"
		return result
	}
	if result.CoverageBps < policy.MinCoverageBps {
		result.Allowed = false
		result.Reason = "coverage"
		return result
	}
	if p.StrictReconcile && result.DriftBps > policy.MaxDriftBps {
		result.Allowed = false
		result.Reason = "debt_drift"
		return result
	}
	result.Allowed = true
	result.Reason = "accepted"
	return result
}

func (p RiskPolicy) SortedProducts() []ProductPolicy {
	keys := make([]string, 0, len(p.Products))
	lookup := make(map[string]ProductKind)
	for key := range p.Products {
		s := string(key)
		keys = append(keys, s)
		lookup[s] = key
	}
	sort.Strings(keys)
	out := make([]ProductPolicy, 0, len(keys))
	for _, key := range keys {
		out = append(out, p.Products[lookup[key]])
	}
	return out
}

func (p *RiskPolicy) SetStrict(strict bool) {
	p.StrictReconcile = strict
}

func (p *RiskPolicy) SetProductEnabled(product ProductKind, enabled bool) {
	item := p.PolicyFor(product)
	item.Enabled = enabled
	if p.Products == nil {
		p.Products = make(map[ProductKind]ProductPolicy)
	}
	p.Products[product] = item
}

func (p *RiskPolicy) SetCoverage(product ProductKind, bps int) {
	item := p.PolicyFor(product)
	item.MinCoverageBps = ClampInt(bps, 0, 50_000)
	if p.Products == nil {
		p.Products = make(map[ProductKind]ProductPolicy)
	}
	p.Products[product] = item
}

func (p ProductPolicy) Describe() string {
	state := "disabled"
	if p.Enabled {
		state = "enabled"
	}
	netting := "isolated"
	if p.AllowNetting {
		netting = "netting"
	}
	return fmt.Sprintf("%s:%s:%s:%d", p.Product, state, netting, p.MinCoverageBps)
}

func ExplainAdmission(result AdmissionResult) string {
	if result.Allowed {
		return fmt.Sprintf("%s %s accepted for %s %s amount=%d", result.Product, result.ProductID, result.Account, result.Asset, result.Amount)
	}
	return fmt.Sprintf("%s %s rejected: %s", result.Product, result.ProductID, result.Reason)
}

func AdmissionFromState(product ProductKind, productID string, account string, asset string, amount Amount, debt *DebtEngine, ledger *Ledger) AdmissionContext {
	return AdmissionContext{
		Product:    product,
		ProductID:  productID,
		Account:    account,
		Asset:      asset,
		Amount:     amount,
		LocalDebt:  debt.TotalLocalForAccountAsset(account, asset),
		GlobalDebt: ledger.GlobalDebt(account, asset),
		Collateral: ledger.AccountBalance(account, asset),
		Operation:  "generic",
	}
}

func PolicyDigest(policy RiskPolicy) string {
	parts := []string{
		fmt.Sprintf("cap:%d", policy.GlobalDebtCap),
		fmt.Sprintf("dust:%d", policy.DustThreshold),
		"strict:" + BoolString(policy.StrictReconcile),
	}
	for _, product := range policy.SortedProducts() {
		parts = append(parts, product.Describe())
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func MergePolicy(base RiskPolicy, overlay RiskPolicy) RiskPolicy {
	out := base
	if out.Products == nil {
		out.Products = make(map[ProductKind]ProductPolicy)
	} else {
		copyMap := make(map[ProductKind]ProductPolicy, len(out.Products))
		for key, value := range out.Products {
			copyMap[key] = value
		}
		out.Products = copyMap
	}
	for key, value := range overlay.Products {
		out.Products[key] = value
	}
	if overlay.GlobalDebtCap != 0 {
		out.GlobalDebtCap = overlay.GlobalDebtCap
	}
	if overlay.DustThreshold != 0 {
		out.DustThreshold = overlay.DustThreshold
	}
	if overlay.StrictReconcile {
		out.StrictReconcile = true
	}
	return out
}

func ReconcileAdmission(policy RiskPolicy, line ReconciliationLine) AdmissionResult {
	return policy.Admit(AdmissionContext{
		Product:    ProductSystem,
		ProductID:  "reconcile",
		Account:    line.Account,
		Asset:      line.Asset,
		Amount:     Positive(line.Drift),
		LocalDebt:  line.LocalOutstanding,
		GlobalDebt: line.GlobalDebt,
		Collateral: line.GlobalDebt,
		Operation:  "reconcile",
	})
}

func PolicyWarnings(policy RiskPolicy) []string {
	warnings := make([]string, 0)
	for _, product := range policy.SortedProducts() {
		if !product.Enabled {
			warnings = append(warnings, fmt.Sprintf("%s disabled", product.Product))
		}
		if product.MinCoverageBps < 10_000 {
			warnings = append(warnings, fmt.Sprintf("%s low coverage", product.Product))
		}
		if product.AllowNetting && product.MaxDriftBps == 0 {
			warnings = append(warnings, fmt.Sprintf("%s allows netting without drift tolerance", product.Product))
		}
	}
	return warnings
}
