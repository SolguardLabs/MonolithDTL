package main

import (
	"fmt"
	"sort"
)

type EscrowBook struct {
	ledger    *Ledger
	debt      *DebtEngine
	operators *OperatorRegistry
	escrows   map[string]*Escrow
}

func NewEscrowBook(ledger *Ledger, debt *DebtEngine, operators *OperatorRegistry) *EscrowBook {
	return &EscrowBook{
		ledger:    ledger,
		debt:      debt,
		operators: operators,
		escrows:   make(map[string]*Escrow),
	}
}

func (b *EscrowBook) Create(id string, depositor string, beneficiary string, asset string, amount Amount, fee Amount, operatorID string, ttl int64) (*Escrow, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("escrow amount must be positive")
	}
	if _, ok := b.escrows[id]; ok {
		return nil, fmt.Errorf("escrow %s exists", id)
	}
	if err := b.operators.Require(operatorID, ProductEscrow, amount, id+":create"); err != nil {
		return nil, err
	}
	if err := b.ledger.Move(ProductEscrow, id, depositor, "reserve-escrow", asset, amount+fee, id, "escrow funded"); err != nil {
		return nil, err
	}
	now := b.ledger.Clock()
	escrow := &Escrow{
		ID:          id,
		Depositor:   depositor,
		Beneficiary: beneficiary,
		Asset:       asset,
		Amount:      amount,
		Fee:         fee,
		State:       "funded",
		OperatorID:  operatorID,
		CreatedAt:   now,
		ExpiresAt:   now + ttl,
	}
	b.escrows[id] = escrow
	if _, err := b.debt.OpenObligation(ProductEscrow, id, depositor, asset, amount, amount+fee, id+":obligation"); err != nil {
		return nil, err
	}
	b.ledger.Emit("escrow_created", ProductEscrow, id, depositor, asset, amount, "info", "escrow created", map[string]interface{}{
		"beneficiary": beneficiary,
		"fee":         fee,
	})
	return escrow, nil
}

func (b *EscrowBook) Release(id string, amount Amount, ref string) error {
	escrow, ok := b.escrows[id]
	if !ok {
		return fmt.Errorf("unknown escrow %s", id)
	}
	if escrow.State != "funded" && escrow.State != "partial" {
		return fmt.Errorf("escrow %s not releasable", id)
	}
	remaining := escrow.Amount - escrow.Released - escrow.Refunded
	applied := MinAmount(remaining, amount)
	if applied <= 0 {
		return fmt.Errorf("nothing to release")
	}
	if err := b.operators.Require(escrow.OperatorID, ProductEscrow, applied, ref); err != nil {
		return err
	}
	if err := b.ledger.Move(ProductEscrow, id, "reserve-escrow", escrow.Beneficiary, escrow.Asset, applied, ref, "escrow release"); err != nil {
		return err
	}
	escrow.Released += applied
	if err := b.debt.ApplyProductCredit(ProductEscrow, id, escrow.Depositor, escrow.Asset, applied, ref+":credit"); err != nil {
		return err
	}
	if escrow.Released+escrow.Refunded >= escrow.Amount {
		escrow.State = "released"
	} else {
		escrow.State = "partial"
	}
	b.ledger.Emit("escrow_released", ProductEscrow, id, escrow.Depositor, escrow.Asset, applied, "info", "escrow released", map[string]interface{}{
		"beneficiary": escrow.Beneficiary,
	})
	return nil
}

func (b *EscrowBook) SettleRefund(id string, amount Amount, ref string) error {
	escrow, ok := b.escrows[id]
	if !ok {
		return fmt.Errorf("unknown escrow %s", id)
	}
	if escrow.State != "funded" && escrow.State != "partial" {
		return fmt.Errorf("escrow %s not refundable", id)
	}
	remaining := escrow.Amount - escrow.Released - escrow.Refunded
	applied := MinAmount(remaining, amount)
	if applied <= 0 {
		return fmt.Errorf("nothing to refund")
	}
	if err := b.operators.Require(escrow.OperatorID, ProductEscrow, applied, ref); err != nil {
		return err
	}
	if err := b.ledger.Move(ProductEscrow, id, "reserve-escrow", escrow.Depositor, escrow.Asset, applied, ref, "escrow refund"); err != nil {
		return err
	}
	escrow.Refunded += applied
	credit := applied + escrow.Fee
	if err := b.debt.ApplyProductCredit(ProductEscrow, id, escrow.Depositor, escrow.Asset, credit, ref+":credit"); err != nil {
		return err
	}
	if escrow.Released+escrow.Refunded >= escrow.Amount {
		escrow.State = "refunded"
	} else {
		escrow.State = "partial"
	}
	b.ledger.Emit("escrow_refunded", ProductEscrow, id, escrow.Depositor, escrow.Asset, applied, "info", "escrow refunded", map[string]interface{}{
		"remaining":    remaining - applied,
		"gross_credit": credit,
	})
	return nil
}

func (b *EscrowBook) CaptureFee(id string, treasury string, ref string) error {
	escrow, ok := b.escrows[id]
	if !ok {
		return fmt.Errorf("unknown escrow %s", id)
	}
	if escrow.Fee <= 0 {
		return nil
	}
	if err := b.ledger.Move(ProductEscrow, id, "reserve-escrow", treasury, escrow.Asset, escrow.Fee, ref, "escrow fee capture"); err != nil {
		return err
	}
	b.ledger.Emit("escrow_fee_captured", ProductEscrow, id, treasury, escrow.Asset, escrow.Fee, "info", "escrow fee captured", nil)
	return nil
}

func (b *EscrowBook) CancelExpired(id string, ref string) error {
	escrow, ok := b.escrows[id]
	if !ok {
		return fmt.Errorf("unknown escrow %s", id)
	}
	if b.ledger.Clock() <= escrow.ExpiresAt {
		return fmt.Errorf("escrow %s not expired", id)
	}
	if escrow.State == "released" || escrow.State == "refunded" {
		return nil
	}
	remaining := escrow.Amount - escrow.Released - escrow.Refunded
	if remaining > 0 {
		if err := b.SettleRefund(id, remaining, ref); err != nil {
			return err
		}
	}
	escrow.State = "cancelled"
	b.ledger.Emit("escrow_cancelled", ProductEscrow, id, escrow.Depositor, escrow.Asset, remaining, "info", "escrow cancelled", nil)
	return nil
}

func (b *EscrowBook) Escrows() []Escrow {
	keys := make([]string, 0, len(b.escrows))
	for key := range b.escrows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Escrow, 0, len(keys))
	for _, key := range keys {
		out = append(out, *b.escrows[key])
	}
	return out
}

func (b *EscrowBook) ExecutedCount() int {
	count := 0
	for _, escrow := range b.escrows {
		if escrow.Released > 0 || escrow.Refunded > 0 {
			count++
		}
	}
	return count
}

func (b *EscrowBook) LockedReserve(asset string) Amount {
	total := Amount(0)
	for _, escrow := range b.escrows {
		if escrow.Asset == asset && (escrow.State == "funded" || escrow.State == "partial") {
			total += escrow.Amount + escrow.Fee - escrow.Released - escrow.Refunded
		}
	}
	return total
}

func (b *EscrowBook) OpenExposure(account string, asset string) Amount {
	total := Amount(0)
	for _, escrow := range b.escrows {
		if escrow.Depositor == account && escrow.Asset == asset {
			total += Positive(escrow.Amount - escrow.Released - escrow.Refunded)
		}
	}
	return total
}
