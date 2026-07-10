package main

import (
	"fmt"
	"sort"
)

type Ledger struct {
	networkID string
	clock     int64
	assets    map[string]Asset
	accounts  map[string]*Account
	entries   []LedgerEntry
	debt      map[string]Amount
	events    []Event
}

func NewLedger(networkID string) *Ledger {
	return &Ledger{
		networkID: networkID,
		assets:    make(map[string]Asset),
		accounts:  make(map[string]*Account),
		entries:   make([]LedgerEntry, 0),
		debt:      make(map[string]Amount),
		events:    make([]Event, 0),
	}
}

func (l *Ledger) Tick() int64 {
	l.clock++
	return l.clock
}

func (l *Ledger) Clock() int64 {
	return l.clock
}

func (l *Ledger) AddAsset(asset Asset) {
	if asset.ID == "" {
		asset.ID = asset.Symbol
	}
	if asset.RiskWeightBps == 0 {
		asset.RiskWeightBps = BpsDenominator
	}
	asset.Enabled = true
	l.assets[asset.ID] = asset
}

func (l *Ledger) Asset(id string) (Asset, bool) {
	asset, ok := l.assets[id]
	return asset, ok
}

func (l *Ledger) Assets() []Asset {
	keys := make([]string, 0, len(l.assets))
	for key := range l.assets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Asset, 0, len(keys))
	for _, key := range keys {
		out = append(out, l.assets[key])
	}
	return out
}

func (l *Ledger) OpenAccount(id string, owner string, role AccountRole, tags ...string) *Account {
	if existing, ok := l.accounts[id]; ok {
		return existing
	}
	account := &Account{
		ID:       id,
		Owner:    owner,
		Role:     role,
		Balances: make(map[string]Amount),
		Tags:     CopyStrings(tags),
	}
	l.accounts[id] = account
	l.Emit("account_opened", ProductSystem, id, id, "", 0, "info", "account opened", map[string]interface{}{
		"owner": owner,
		"role":  string(role),
	})
	return account
}

func (l *Ledger) Account(id string) (*Account, bool) {
	account, ok := l.accounts[id]
	return account, ok
}

func (l *Ledger) MustAccount(id string) *Account {
	account, ok := l.accounts[id]
	if !ok {
		panic(fmt.Sprintf("missing account %s", id))
	}
	return account
}

func (l *Ledger) Credit(product ProductKind, productID string, accountID string, asset string, amount Amount, ref string, memo string) error {
	if err := MustPositive("credit amount", amount); err != nil {
		return err
	}
	if err := MustKnownAsset(l.assets, asset); err != nil {
		return err
	}
	account, ok := l.accounts[accountID]
	if !ok {
		return fmt.Errorf("unknown account %s", accountID)
	}
	account.Balances[asset] = Add(account.Balances[asset], amount)
	l.appendEntry(product, productID, accountID, asset, 0, amount, 0, DebtKey(accountID, asset), ref, memo)
	return nil
}

func (l *Ledger) Debit(product ProductKind, productID string, accountID string, asset string, amount Amount, ref string, memo string) error {
	if err := MustPositive("debit amount", amount); err != nil {
		return err
	}
	if err := MustKnownAsset(l.assets, asset); err != nil {
		return err
	}
	account, ok := l.accounts[accountID]
	if !ok {
		return fmt.Errorf("unknown account %s", accountID)
	}
	if account.Frozen {
		return fmt.Errorf("account %s frozen", accountID)
	}
	if account.Balances[asset] < amount {
		return fmt.Errorf("insufficient balance %s %s: have %d need %d", accountID, asset, account.Balances[asset], amount)
	}
	account.Balances[asset] = Sub(account.Balances[asset], amount)
	l.appendEntry(product, productID, accountID, asset, amount, 0, 0, DebtKey(accountID, asset), ref, memo)
	return nil
}

func (l *Ledger) Move(product ProductKind, productID string, from string, to string, asset string, amount Amount, ref string, memo string) error {
	if amount <= 0 {
		return fmt.Errorf("move amount must be positive")
	}
	if err := l.Debit(product, productID, from, asset, amount, ref, memo+":debit"); err != nil {
		return err
	}
	if err := l.Credit(product, productID, to, asset, amount, ref, memo+":credit"); err != nil {
		return err
	}
	l.Emit("ledger_move", product, productID, from, asset, amount, "info", memo, map[string]interface{}{
		"to":  to,
		"ref": ref,
	})
	return nil
}

func (l *Ledger) ReserveDebt(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	if amount <= 0 {
		return fmt.Errorf("debt reserve must be positive")
	}
	if err := MustKnownAsset(l.assets, asset); err != nil {
		return err
	}
	key := DebtKey(account, asset)
	l.debt[key] = Add(l.debt[key], amount)
	l.appendEntry(product, productID, account, asset, 0, 0, amount, key, ref, "global debt reserve")
	l.Emit("global_debt_reserved", product, productID, account, asset, amount, "info", "global debt reserved", map[string]interface{}{
		"key": key,
	})
	return nil
}

func (l *Ledger) RepayDebt(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	if amount <= 0 {
		return fmt.Errorf("debt repayment must be positive")
	}
	key := DebtKey(account, asset)
	current := l.debt[key]
	applied := MinAmount(current, amount)
	l.debt[key] = Sub(current, applied)
	l.appendEntry(product, productID, account, asset, 0, 0, -applied, key, ref, "global debt repayment")
	l.Emit("global_debt_repaid", product, productID, account, asset, applied, "info", "global debt repaid", map[string]interface{}{
		"key":       key,
		"requested": amount,
		"remaining": l.debt[key],
	})
	return nil
}

func (l *Ledger) ReleaseDebt(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	if amount <= 0 {
		return fmt.Errorf("debt release must be positive")
	}
	key := DebtKey(account, asset)
	current := l.debt[key]
	applied := MinAmount(current, amount)
	l.debt[key] = Sub(current, applied)
	l.appendEntry(product, productID, account, asset, 0, 0, -applied, key, ref, "global debt release")
	l.Emit("global_debt_released", product, productID, account, asset, applied, "info", "global debt released", map[string]interface{}{
		"key":       key,
		"requested": amount,
		"remaining": l.debt[key],
	})
	return nil
}

func (l *Ledger) GlobalDebt(account string, asset string) Amount {
	return l.debt[DebtKey(account, asset)]
}

func (l *Ledger) TotalGlobalDebt() Amount {
	total := Amount(0)
	for _, amount := range l.debt {
		total += amount
	}
	return total
}

func (l *Ledger) DebtViews() []GlobalDebtView {
	keys := make([]string, 0, len(l.debt))
	for key := range l.debt {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]GlobalDebtView, 0, len(keys))
	for _, key := range keys {
		parts := splitDebtKey(key)
		out = append(out, GlobalDebtView{
			Account: parts[0],
			Asset:   parts[1],
			Amount:  l.debt[key],
		})
	}
	return out
}

func (l *Ledger) BalanceViews() []BalanceView {
	accountKeys := make([]string, 0, len(l.accounts))
	for key := range l.accounts {
		accountKeys = append(accountKeys, key)
	}
	sort.Strings(accountKeys)
	out := make([]BalanceView, 0)
	for _, accountID := range accountKeys {
		account := l.accounts[accountID]
		assetKeys := make([]string, 0, len(account.Balances))
		for asset := range account.Balances {
			assetKeys = append(assetKeys, asset)
		}
		sort.Strings(assetKeys)
		for _, asset := range assetKeys {
			out = append(out, BalanceView{
				Account: account.ID,
				Owner:   account.Owner,
				Role:    string(account.Role),
				Asset:   asset,
				Amount:  account.Balances[asset],
				Frozen:  account.Frozen,
			})
		}
	}
	return out
}

func (l *Ledger) Entries() []LedgerEntry {
	out := make([]LedgerEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

func (l *Ledger) Events() []Event {
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}

func (l *Ledger) Emit(kind string, product ProductKind, productID string, account string, asset string, amount Amount, severity string, message string, attrs map[string]interface{}) {
	clock := l.Tick()
	event := Event{
		ID:        StableID("evt", kind, productID, account, asset, AmountToString(amount), AmountToString(Amount(clock))),
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
	l.events = append(l.events, event)
}

func (l *Ledger) appendEntry(product ProductKind, productID string, account string, asset string, debit Amount, credit Amount, debtDelta Amount, bucket string, ref string, memo string) {
	clock := l.Tick()
	entry := LedgerEntry{
		ID:        StableID("led", string(product), productID, account, asset, AmountToString(Amount(clock)), ref, memo),
		Clock:     clock,
		Product:   product,
		ProductID: productID,
		Account:   account,
		Asset:     asset,
		Debit:     debit,
		Credit:    credit,
		DebtDelta: debtDelta,
		Bucket:    bucket,
		Ref:       ref,
		Memo:      memo,
	}
	l.entries = append(l.entries, entry)
}

func (l *Ledger) Freeze(accountID string, reason string) error {
	account, ok := l.accounts[accountID]
	if !ok {
		return fmt.Errorf("unknown account %s", accountID)
	}
	account.Frozen = true
	l.Emit("account_frozen", ProductSystem, accountID, accountID, "", 0, "warning", reason, nil)
	return nil
}

func (l *Ledger) Unfreeze(accountID string, reason string) error {
	account, ok := l.accounts[accountID]
	if !ok {
		return fmt.Errorf("unknown account %s", accountID)
	}
	account.Frozen = false
	l.Emit("account_unfrozen", ProductSystem, accountID, accountID, "", 0, "info", reason, nil)
	return nil
}

func (l *Ledger) LedgerNonNegative() bool {
	for _, account := range l.accounts {
		for _, amount := range account.Balances {
			if amount < 0 {
				return false
			}
		}
	}
	for _, debt := range l.debt {
		if debt < 0 {
			return false
		}
	}
	return true
}

func (l *Ledger) SnapshotDigest() string {
	parts := make([]string, 0)
	for _, asset := range l.Assets() {
		parts = append(parts, "asset:"+asset.ID+":"+asset.Symbol)
	}
	for _, balance := range l.BalanceViews() {
		parts = append(parts, fmt.Sprintf("balance:%s:%s:%d", balance.Account, balance.Asset, balance.Amount))
	}
	for _, debt := range l.DebtViews() {
		parts = append(parts, fmt.Sprintf("debt:%s:%s:%d", debt.Account, debt.Asset, debt.Amount))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func splitDebtKey(key string) [2]string {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}

func (l *Ledger) AccountBalance(account string, asset string) Amount {
	if a, ok := l.accounts[account]; ok {
		return a.Balances[asset]
	}
	return 0
}

func (l *Ledger) Mint(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	return l.Credit(product, productID, account, asset, amount, ref, "mint")
}

func (l *Ledger) Burn(product ProductKind, productID string, account string, asset string, amount Amount, ref string) error {
	return l.Debit(product, productID, account, asset, amount, ref, "burn")
}

func (l *Ledger) AccountsByRole(role AccountRole) []string {
	keys := make([]string, 0)
	for id, account := range l.accounts {
		if account.Role == role {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys)
	return keys
}

func (l *Ledger) SumBalances(asset string, role AccountRole) Amount {
	total := Amount(0)
	for _, account := range l.accounts {
		if account.Role == role {
			total += account.Balances[asset]
		}
	}
	return total
}

func (l *Ledger) HasAccount(id string) bool {
	_, ok := l.accounts[id]
	return ok
}

func (l *Ledger) EnsureBalance(account string, asset string, amount Amount) bool {
	return l.AccountBalance(account, asset) >= amount
}

