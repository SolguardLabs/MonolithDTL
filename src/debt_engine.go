package main

import (
	"fmt"
	"sort"
)

type DebtEngine struct {
	ledger       *Ledger
	local        map[string]*LocalSolvency
	productIndex map[string][]string
	events       []Event
}

func NewDebtEngine(ledger *Ledger) *DebtEngine {
	return &DebtEngine{
		ledger:       ledger,
		local:        make(map[string]*LocalSolvency),
		productIndex: make(map[string][]string),
		events:       make([]Event, 0),
	}
}

func (d *DebtEngine) OpenObligation(product ProductKind, productID string, account string, asset string, obligation Amount, collateral Amount, ref string) (*LocalSolvency, error) {
	if obligation <= 0 {
		return nil, fmt.Errorf("obligation must be positive")
	}
	key := LocalDebtKey(product, productID, account, asset)
	if existing, ok := d.local[key]; ok {
		existing.Obligation += obligation
		existing.Collateral += collateral
		existing.Reserved += obligation
		existing.State = "open"
		existing.LastEvent = ref
		if err := d.ledger.ReserveDebt(product, productID, account, asset, obligation, ref); err != nil {
			return nil, err
		}
		d.emit("local_obligation_increased", product, productID, account, asset, obligation, "info", "local obligation increased", map[string]interface{}{
			"key": key,
		})
		return existing, nil
	}
	line := &LocalSolvency{
		Product:    product,
		ProductID:  productID,
		Account:    account,
		Asset:      asset,
		Obligation: obligation,
		Collateral:  collateral,
		Reserved:    obligation,
		State:       "open",
		LastEvent:   ref,
	}
	d.local[key] = line
	indexKey := ProductKey(product, productID)
	d.productIndex[indexKey] = append(d.productIndex[indexKey], key)
	if err := d.ledger.ReserveDebt(product, productID, account, asset, obligation, ref); err != nil {
		return nil, err
	}
	d.emit("local_obligation_opened", product, productID, account, asset, obligation, "info", "local obligation opened", map[string]interface{}{
		"key":        key,
		"collateral": collateral,
	})
	return line, nil
}

func (d *DebtEngine) ApplyProductCredit(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	if amount <= 0 {
		return fmt.Errorf("credit amount must be positive")
	}
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return fmt.Errorf("missing local obligation for %s %s %s %s", product, productID, account, asset)
	}
	appliedLocal := MinAmount(line.Outstanding(), amount)
	line.Settled += appliedLocal
	line.LastEvent = ref
	if line.Outstanding() == 0 {
		line.State = "settled"
	} else {
		line.State = "partial"
	}
	if err := d.ledger.RepayDebt(product, productID, account, asset, amount, ref); err != nil {
		return err
	}
	d.emit("product_credit_applied", product, productID, account, asset, amount, "info", "product credit applied to shared debt", map[string]interface{}{
		"local_applied": appliedLocal,
		"local_open":    line.Outstanding(),
		"global_open":   d.ledger.GlobalDebt(account, asset),
	})
	return nil
}

func (d *DebtEngine) ReserveLocal(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return fmt.Errorf("missing local obligation")
	}
	line.Reserved += amount
	line.LastEvent = ref
	d.emit("local_reserved", product, productID, account, asset, amount, "info", "local reserve updated", nil)
	return nil
}

func (d *DebtEngine) ReleaseLocal(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return fmt.Errorf("missing local obligation")
	}
	applied := MinAmount(line.Reserved, amount)
	line.Reserved -= applied
	line.LastEvent = ref
	d.emit("local_released", product, productID, account, asset, applied, "info", "local reserve released", nil)
	return nil
}

func (d *DebtEngine) MarkRedeemed(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return fmt.Errorf("missing local obligation")
	}
	line.Redeemed += amount
	line.LastEvent = ref
	if line.Outstanding() > 0 {
		line.LocalOnlyNotes = append(line.LocalOnlyNotes, "redeemed while local debt remains open")
	}
	d.emit("local_redeemed", product, productID, account, asset, amount, "warning", "local line marked redeemed", map[string]interface{}{
		"outstanding": line.Outstanding(),
	})
	return nil
}

func (d *DebtEngine) Liquidate(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return fmt.Errorf("missing local obligation")
	}
	applied := MinAmount(line.Outstanding(), amount)
	line.Liquidated += applied
	line.Settled += applied
	line.LastEvent = ref
	if line.Outstanding() == 0 {
		line.State = "liquidated"
	}
	if err := d.ledger.ReleaseDebt(product, productID, account, asset, applied, ref); err != nil {
		return err
	}
	d.emit("local_liquidated", product, productID, account, asset, applied, "warning", "local debt liquidated", nil)
	return nil
}

func (d *DebtEngine) LocalOutstanding(product ProductKind, productID string, account string, asset string) Amount {
	line := d.findLocal(product, productID, account, asset)
	if line == nil {
		return 0
	}
	return line.Outstanding()
}

func (d *DebtEngine) TotalLocalOutstanding() Amount {
	total := Amount(0)
	for _, line := range d.local {
		total += line.Outstanding()
	}
	return total
}

func (d *DebtEngine) TotalLocalForAccountAsset(account string, asset string) Amount {
	total := Amount(0)
	for _, line := range d.local {
		if line.Account == account && line.Asset == asset {
			total += line.Outstanding()
		}
	}
	return total
}

func (d *DebtEngine) OutstandingByProduct(product ProductKind) Amount {
	total := Amount(0)
	for _, line := range d.local {
		if line.Product == product {
			total += line.Outstanding()
		}
	}
	return total
}

func (d *DebtEngine) LocalViews() []LocalDebtView {
	keys := make([]string, 0, len(d.local))
	for key := range d.local {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]LocalDebtView, 0, len(keys))
	for _, key := range keys {
		line := d.local[key]
		out = append(out, LocalDebtView{
			Key:         key,
			Product:     line.Product,
			ProductID:   line.ProductID,
			Account:     line.Account,
			Asset:       line.Asset,
			Outstanding: line.Outstanding(),
			Obligation:  line.Obligation,
			Settled:      line.Settled,
			Collateral:   line.Collateral,
			State:       line.State,
		})
	}
	return out
}

func (d *DebtEngine) LinesForProduct(product ProductKind, productID string) []*LocalSolvency {
	indexKey := ProductKey(product, productID)
	keys := append([]string(nil), d.productIndex[indexKey]...)
	sort.Strings(keys)
	out := make([]*LocalSolvency, 0, len(keys))
	for _, key := range keys {
		if line, ok := d.local[key]; ok {
			out = append(out, line)
		}
	}
	return out
}

func (d *DebtEngine) Reconcile() []ReconciliationLine {
	seen := make(map[string]bool)
	for _, line := range d.local {
		seen[DebtKey(line.Account, line.Asset)] = true
	}
	for _, debt := range d.ledger.DebtViews() {
		seen[DebtKey(debt.Account, debt.Asset)] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ReconciliationLine, 0, len(keys))
	for _, key := range keys {
		parts := splitDebtKey(key)
		local := d.TotalLocalForAccountAsset(parts[0], parts[1])
		global := d.ledger.GlobalDebt(parts[0], parts[1])
		drift := local - global
		severity := "ok"
		if drift != 0 {
			severity = "warning"
		}
		if drift > 0 {
			severity = "critical"
		}
		out = append(out, ReconciliationLine{
			Account:          parts[0],
			Asset:            parts[1],
			LocalOutstanding: local,
			GlobalDebt:       global,
			Drift:            drift,
			Severity:         severity,
		})
	}
	return out
}

func (d *DebtEngine) HasCrossProductDrift() bool {
	for _, line := range d.Reconcile() {
		if line.Drift != 0 {
			return true
		}
	}
	return false
}

func (d *DebtEngine) PhantomDebt() Amount {
	total := Amount(0)
	for _, line := range d.Reconcile() {
		if line.Drift > 0 {
			total += line.Drift
		}
	}
	return total
}

func (d *DebtEngine) LocalSolvencyNonNegative() bool {
	for _, line := range d.local {
		if line.Obligation < 0 || line.Settled < 0 || line.Collateral < 0 || line.Outstanding() < 0 {
			return false
		}
	}
	return true
}

func (d *DebtEngine) ProductsHaveLocalState() bool {
	for _, line := range d.local {
		if line.Product == "" || line.ProductID == "" || line.Account == "" || line.Asset == "" {
			return false
		}
	}
	return true
}

func (d *DebtEngine) Events() []Event {
	out := make([]Event, len(d.events))
	copy(out, d.events)
	return out
}

func (d *DebtEngine) findLocal(product ProductKind, productID string, account string, asset string) *LocalSolvency {
	key := LocalDebtKey(product, productID, account, asset)
	return d.local[key]
}

func (d *DebtEngine) emit(kind string, product ProductKind, productID string, account string, asset string, amount Amount, severity string, message string, attrs map[string]interface{}) {
	clock := d.ledger.Tick()
	event := Event{
		ID:        StableID("evt", kind, string(product), productID, account, asset, AmountToString(amount), AmountToString(Amount(clock))),
		Clock:     clock,
		Kind:      kind,
		Product:   product,
		ProductID: productID,
		Account:   account,
		Asset:     asset,
		Amount:    amount,
		Severity:  severity,
		Message:   message,
		Attrs:     attrs,
	}
	d.events = append(d.events, event)
}

func (l *LocalSolvency) Outstanding() Amount {
	return Positive(l.Obligation - l.Settled - l.Liquidated)
}

func (l *LocalSolvency) CoverageBps() int {
	return RatioBps(l.Collateral, MaxAmountOf(l.Outstanding(), 1))
}

func (l *LocalSolvency) IsSolvent(minCoverageBps int) bool {
	if l.Outstanding() == 0 {
		return true
	}
	return l.CoverageBps() >= minCoverageBps
}

func (l *LocalSolvency) Clone() LocalSolvency {
	out := *l
	out.LocalOnlyNotes = CopyStrings(l.LocalOnlyNotes)
	return out
}

func (d *DebtEngine) AccountRisk(account string, asset string) map[string]Amount {
	return map[string]Amount{
		"local":  d.TotalLocalForAccountAsset(account, asset),
		"global": d.ledger.GlobalDebt(account, asset),
		"drift":  d.TotalLocalForAccountAsset(account, asset) - d.ledger.GlobalDebt(account, asset),
	}
}

func (d *DebtEngine) RequireNoDrift(account string, asset string) error {
	risk := d.AccountRisk(account, asset)
	if risk["drift"] != 0 {
		return fmt.Errorf("debt drift for %s %s: local=%d global=%d", account, asset, risk["local"], risk["global"])
	}
	return nil
}

