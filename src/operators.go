package main

import (
	"fmt"
	"sort"
)

type OperatorRegistry struct {
	ledger    *Ledger
	operators map[string]*Operator
}

func NewOperatorRegistry(ledger *Ledger) *OperatorRegistry {
	return &OperatorRegistry{
		ledger:    ledger,
		operators: make(map[string]*Operator),
	}
}

func (r *OperatorRegistry) Register(id string, account string, name string, dailyLimit Amount, tier string, allowed ...ProductKind) (*Operator, error) {
	if id == "" {
		return nil, fmt.Errorf("operator id required")
	}
	if _, ok := r.operators[id]; ok {
		return nil, fmt.Errorf("operator %s already exists", id)
	}
	if !r.ledger.HasAccount(account) {
		return nil, fmt.Errorf("operator account %s missing", account)
	}
	op := &Operator{
		ID:          id,
		Account:     account,
		Name:        name,
		Active:      true,
		Allowed:     CopyProducts(allowed),
		DailyLimit:  dailyLimit,
		RiskTier:    tier,
		ProductCaps: make(map[ProductKind]int),
	}
	for _, product := range allowed {
		op.ProductCaps[product] = 10_000
	}
	r.operators[id] = op
	r.ledger.Emit("operator_registered", ProductOperator, id, account, "", dailyLimit, "info", "operator registered", map[string]interface{}{
		"tier": tier,
	})
	return op, nil
}

func (r *OperatorRegistry) Authorize(id string, product ProductKind, capBps int) error {
	op, ok := r.operators[id]
	if !ok {
		return fmt.Errorf("unknown operator %s", id)
	}
	if !ProductAllowed(product, op.Allowed) {
		op.Allowed = append(op.Allowed, product)
	}
	op.ProductCaps[product] = ClampInt(capBps, 0, 10_000)
	r.ledger.Emit("operator_authorized", ProductOperator, id, op.Account, "", 0, "info", "operator authorized", map[string]interface{}{
		"product": string(product),
		"cap_bps": op.ProductCaps[product],
	})
	return nil
}

func (r *OperatorRegistry) Use(id string, product ProductKind, amount Amount, ref string) error {
	op, ok := r.operators[id]
	if !ok {
		return fmt.Errorf("unknown operator %s", id)
	}
	if !op.Active {
		return fmt.Errorf("operator %s inactive", id)
	}
	if !ProductAllowed(product, op.Allowed) {
		return fmt.Errorf("operator %s not allowed for %s", id, product)
	}
	next := op.UsedToday + amount
	if next > op.DailyLimit {
		return fmt.Errorf("operator %s over daily limit", id)
	}
	op.UsedToday = next
	op.LastSettlement = ref
	r.ledger.Emit("operator_limit_used", ProductOperator, id, op.Account, "", amount, "info", "operator limit used", map[string]interface{}{
		"product": string(product),
		"used":    op.UsedToday,
		"limit":   op.DailyLimit,
	})
	return nil
}

func (r *OperatorRegistry) Deactivate(id string, reason string) error {
	op, ok := r.operators[id]
	if !ok {
		return fmt.Errorf("unknown operator %s", id)
	}
	op.Active = false
	r.ledger.Emit("operator_deactivated", ProductOperator, id, op.Account, "", 0, "warning", reason, nil)
	return nil
}

func (r *OperatorRegistry) ResetDay() {
	keys := make([]string, 0, len(r.operators))
	for key := range r.operators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r.operators[key].UsedToday = 0
		r.ledger.Emit("operator_day_reset", ProductOperator, key, r.operators[key].Account, "", 0, "info", "operator day reset", nil)
	}
}

func (r *OperatorRegistry) Get(id string) (*Operator, bool) {
	op, ok := r.operators[id]
	return op, ok
}

func (r *OperatorRegistry) Views() []OperatorView {
	keys := make([]string, 0, len(r.operators))
	for key := range r.operators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]OperatorView, 0, len(keys))
	for _, key := range keys {
		op := r.operators[key]
		allowed := make([]string, 0, len(op.Allowed))
		for _, product := range op.Allowed {
			allowed = append(allowed, string(product))
		}
		sort.Strings(allowed)
		out = append(out, OperatorView{
			ID:          op.ID,
			Account:     op.Account,
			Name:        op.Name,
			Active:      op.Active,
			Allowed:     allowed,
			DailyLimit:  op.DailyLimit,
			UsedToday:   op.UsedToday,
			RiskTier:    op.RiskTier,
			Utilization: RatioBps(op.UsedToday, MaxAmountOf(op.DailyLimit, 1)),
		})
	}
	return out
}

func (r *OperatorRegistry) OperatorsWithinLimit() bool {
	for _, op := range r.operators {
		if op.UsedToday > op.DailyLimit {
			return false
		}
	}
	return true
}

func (r *OperatorRegistry) Require(id string, product ProductKind, amount Amount, ref string) error {
	return r.Use(id, product, amount, ref)
}

func ClampInt(value int, lo int, hi int) int {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}
