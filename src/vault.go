package main

import (
	"fmt"
	"sort"
)

type VaultBook struct {
	ledger    *Ledger
	debt      *DebtEngine
	operators *OperatorRegistry
	vaults    map[string]*Vault
	positions map[string]*VaultPosition
}

func NewVaultBook(ledger *Ledger, debt *DebtEngine, operators *OperatorRegistry) *VaultBook {
	return &VaultBook{
		ledger:    ledger,
		debt:      debt,
		operators: operators,
		vaults:    make(map[string]*Vault),
		positions: make(map[string]*VaultPosition),
	}
}

func (b *VaultBook) Create(id string, asset string, reserve string, shareAsset string, feeBps int) (*Vault, error) {
	if _, ok := b.vaults[id]; ok {
		return nil, fmt.Errorf("vault %s exists", id)
	}
	if !b.ledger.HasAccount(reserve) {
		return nil, fmt.Errorf("reserve account missing")
	}
	vault := &Vault{
		ID:         id,
		Asset:      asset,
		Reserve:    reserve,
		ShareAsset: shareAsset,
		FeeBps:     feeBps,
	}
	b.vaults[id] = vault
	b.ledger.Emit("vault_created", ProductVault, id, reserve, asset, 0, "info", "vault created", map[string]interface{}{
		"share_asset": shareAsset,
	})
	return vault, nil
}

func (b *VaultBook) Deposit(vaultID string, account string, amount Amount, ref string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	if vault.Paused {
		return fmt.Errorf("vault paused")
	}
	if amount <= 0 {
		return fmt.Errorf("deposit amount must be positive")
	}
	shares := amount
	if vault.TotalShares > 0 && vault.TotalAssets > 0 {
		shares = Amount((int64(amount) * int64(vault.TotalShares)) / int64(vault.TotalAssets))
	}
	if shares <= 0 {
		return fmt.Errorf("deposit would mint zero shares")
	}
	if err := b.ledger.Move(ProductVault, vaultID, account, vault.Reserve, vault.Asset, amount, ref, "vault deposit"); err != nil {
		return err
	}
	if _, ok := b.ledger.Asset(vault.ShareAsset); !ok {
		b.ledger.AddAsset(Asset{ID: vault.ShareAsset, Symbol: vault.ShareAsset, Decimals: 6, RiskWeightBps: 10_000})
	}
	if err := b.ledger.Mint(ProductVault, vaultID, account, vault.ShareAsset, shares, ref+":shares"); err != nil {
		return err
	}
	position := b.position(vaultID, account)
	position.Shares += shares
	position.LastAction = ref
	vault.TotalShares += shares
	vault.TotalAssets += amount
	b.ledger.Emit("vault_deposit", ProductVault, vaultID, account, vault.Asset, amount, "info", "vault deposit accepted", map[string]interface{}{
		"shares": shares,
	})
	return nil
}

func (b *VaultBook) Borrow(vaultID string, account string, amount Amount, operatorID string, ref string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	if amount <= 0 {
		return fmt.Errorf("borrow amount must be positive")
	}
	if err := b.operators.Require(operatorID, ProductVault, amount, ref); err != nil {
		return err
	}
	position := b.position(vaultID, account)
	maxBorrow := ShareOf(vault.TotalAssets, position.Shares, MaxAmountOf(vault.TotalShares, 1))
	maxBorrow = MulBps(maxBorrow, 7_000)
	if position.PrincipalDebt+amount > maxBorrow {
		return fmt.Errorf("borrow exceeds position limit")
	}
	if err := b.ledger.Move(ProductVault, vaultID, vault.Reserve, account, vault.Asset, amount, ref, "vault borrow"); err != nil {
		return err
	}
	position.PrincipalDebt += amount
	position.LastAction = ref
	vault.TotalAssets -= amount
	vault.OpenDebt += amount
	if _, err := b.debt.OpenObligation(ProductVault, vaultID, account, vault.Asset, amount, maxBorrow, ref+":obligation"); err != nil {
		return err
	}
	b.ledger.Emit("vault_borrow", ProductVault, vaultID, account, vault.Asset, amount, "info", "vault borrow executed", map[string]interface{}{
		"max_borrow": maxBorrow,
	})
	return nil
}

func (b *VaultBook) AccrueFee(vaultID string, account string, periods int, ref string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	position := b.position(vaultID, account)
	if position.PrincipalDebt <= 0 || periods <= 0 {
		return nil
	}
	fee := MulBps(position.PrincipalDebt, vault.FeeBps*periods)
	position.AccruedFees += fee
	vault.OpenDebt += fee
	if _, err := b.debt.OpenObligation(ProductVault, vaultID, account, vault.Asset, fee, position.Shares, ref+":fee"); err != nil {
		return err
	}
	b.ledger.Emit("vault_fee_accrued", ProductVault, vaultID, account, vault.Asset, fee, "info", "vault fee accrued", map[string]interface{}{
		"periods": periods,
	})
	return nil
}

func (b *VaultBook) Repay(vaultID string, account string, amount Amount, ref string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	position := b.position(vaultID, account)
	open := position.PrincipalDebt + position.AccruedFees
	applied := MinAmount(open, amount)
	if applied <= 0 {
		return nil
	}
	if err := b.ledger.Move(ProductVault, vaultID, account, vault.Reserve, vault.Asset, applied, ref, "vault repay"); err != nil {
		return err
	}
	if applied <= position.AccruedFees {
		position.AccruedFees -= applied
	} else {
		remaining := applied - position.AccruedFees
		position.AccruedFees = 0
		position.PrincipalDebt -= MinAmount(position.PrincipalDebt, remaining)
	}
	position.LastAction = ref
	vault.TotalAssets += applied
	vault.OpenDebt -= MinAmount(vault.OpenDebt, applied)
	if err := b.debt.ApplyProductCredit(ProductVault, vaultID, account, vault.Asset, applied, ref+":credit"); err != nil {
		return err
	}
	b.ledger.Emit("vault_repay", ProductVault, vaultID, account, vault.Asset, applied, "info", "vault repay applied", nil)
	return nil
}

func (b *VaultBook) Redeem(vaultID string, account string, shares Amount, minOut Amount, ref string) (RedemptionRequest, error) {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return RedemptionRequest{}, fmt.Errorf("unknown vault %s", vaultID)
	}
	position := b.position(vaultID, account)
	request := RedemptionRequest{
		ID:        StableID("red", vaultID, account, AmountToString(shares), ref),
		Product:   ProductVault,
		ProductID: vaultID,
		Account:   account,
		Asset:     vault.Asset,
		Shares:    shares,
		CreatedAt: b.ledger.Clock(),
		Status:    "pending",
	}
	if shares <= 0 {
		request.Status = "rejected"
		request.Reason = "shares_not_positive"
		return request, fmt.Errorf("shares must be positive")
	}
	if position.Shares < shares {
		request.Status = "rejected"
		request.Reason = "insufficient_shares"
		return request, fmt.Errorf("insufficient shares")
	}
	grossOut := ShareOf(vault.TotalAssets, shares, MaxAmountOf(vault.TotalShares, 1))
	fee := MulBps(grossOut, vault.FeeBps)
	out := grossOut - fee
	request.ExpectedOut = out
	globalDebt := b.ledger.GlobalDebt(account, vault.Asset)
	request.BlockedByDebt = globalDebt
	unlocked := Positive(out - globalDebt)
	if unlocked < minOut {
		request.Status = "rejected"
		request.Reason = "blocked_by_global_debt"
		request.ProcessedAt = b.ledger.Clock()
		b.ledger.Emit("vault_redemption_rejected", ProductVault, vaultID, account, vault.Asset, out, "warning", "vault redemption rejected", map[string]interface{}{
			"global_debt": globalDebt,
			"min_out":     minOut,
		})
		return request, fmt.Errorf("redemption blocked by global debt")
	}
	if err := b.ledger.Burn(ProductVault, vaultID, account, vault.ShareAsset, shares, ref+":burn"); err != nil {
		request.Status = "rejected"
		request.Reason = "share_burn_failed"
		return request, err
	}
	if err := b.ledger.Move(ProductVault, vaultID, vault.Reserve, account, vault.Asset, unlocked, ref, "vault redemption"); err != nil {
		request.Status = "rejected"
		request.Reason = "reserve_move_failed"
		return request, err
	}
	position.Shares -= shares
	position.LastAction = ref
	vault.TotalShares -= shares
	vault.TotalAssets -= grossOut
	request.ExpectedOut = unlocked
	request.Status = "processed"
	request.ProcessedAt = b.ledger.Clock()
	localOpen := b.debt.LocalOutstanding(ProductVault, vaultID, account, vault.Asset)
	if localOpen > 0 {
		_ = b.debt.MarkRedeemed(ProductVault, vaultID, account, vault.Asset, unlocked, ref+":local-redeem")
	}
	b.ledger.Emit("vault_redemption_processed", ProductVault, vaultID, account, vault.Asset, unlocked, "info", "vault redemption processed", map[string]interface{}{
		"global_debt": globalDebt,
		"local_debt":  localOpen,
		"fee":         fee,
	})
	return request, nil
}

func (b *VaultBook) Pause(vaultID string, reason string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	vault.Paused = true
	b.ledger.Emit("vault_paused", ProductVault, vaultID, vault.Reserve, vault.Asset, 0, "warning", reason, nil)
	return nil
}

func (b *VaultBook) Unpause(vaultID string, reason string) error {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return fmt.Errorf("unknown vault %s", vaultID)
	}
	vault.Paused = false
	b.ledger.Emit("vault_unpaused", ProductVault, vaultID, vault.Reserve, vault.Asset, 0, "info", reason, nil)
	return nil
}

func (b *VaultBook) Vaults() []Vault {
	keys := make([]string, 0, len(b.vaults))
	for key := range b.vaults {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Vault, 0, len(keys))
	for _, key := range keys {
		out = append(out, *b.vaults[key])
	}
	return out
}

func (b *VaultBook) Positions() []VaultPosition {
	keys := make([]string, 0, len(b.positions))
	for key := range b.positions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]VaultPosition, 0, len(keys))
	for _, key := range keys {
		out = append(out, *b.positions[key])
	}
	return out
}

func (b *VaultBook) position(vaultID string, account string) *VaultPosition {
	key := vaultID + "|" + account
	if pos, ok := b.positions[key]; ok {
		return pos
	}
	pos := &VaultPosition{
		VaultID: vaultID,
		Account: account,
	}
	b.positions[key] = pos
	return pos
}

func (b *VaultBook) LocalDebtRemaining(vaultID string, account string) Amount {
	vault, ok := b.vaults[vaultID]
	if !ok {
		return 0
	}
	return b.debt.LocalOutstanding(ProductVault, vaultID, account, vault.Asset)
}

func (b *VaultBook) ReserveLeak() Amount {
	total := Amount(0)
	for _, vault := range b.vaults {
		for _, pos := range b.positions {
			if pos.VaultID != vault.ID {
				continue
			}
			local := b.debt.LocalOutstanding(ProductVault, vault.ID, pos.Account, vault.Asset)
			global := b.ledger.GlobalDebt(pos.Account, vault.Asset)
			if local > global {
				total += local - global
			}
		}
	}
	return total
}

func (b *VaultBook) RedeemsRespectLocalDebt(redemptions []RedemptionRequest) bool {
	for _, redemption := range redemptions {
		if redemption.Product != ProductVault || redemption.Status != "processed" {
			continue
		}
		if b.LocalDebtRemaining(redemption.ProductID, redemption.Account) > 0 {
			return false
		}
	}
	return true
}

