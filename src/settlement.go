package main

import (
	"fmt"
	"sort"
)

type SettlementAction string

const (
	ActionMove       SettlementAction = "move"
	ActionDebtOpen   SettlementAction = "debt_open"
	ActionDebtCredit SettlementAction = "debt_credit"
	ActionEmit       SettlementAction = "emit"
)

type SettlementCall struct {
	ID        string           `json:"id"`
	Action    SettlementAction `json:"action"`
	Product   ProductKind      `json:"product"`
	ProductID string           `json:"product_id"`
	From      string           `json:"from,omitempty"`
	To        string           `json:"to,omitempty"`
	Account   string           `json:"account,omitempty"`
	Asset     string           `json:"asset"`
	Amount    Amount           `json:"amount"`
	Ref       string           `json:"ref"`
	Memo      string           `json:"memo"`
}

type SettlementBatch struct {
	ID          string           `json:"id"`
	OperatorID  string           `json:"operator_id"`
	Calls       []SettlementCall `json:"calls"`
	Status      string           `json:"status"`
	Failure     string           `json:"failure,omitempty"`
	TotalAmount Amount           `json:"total_amount"`
	CreatedAt   int64            `json:"created_at"`
	SettledAt    int64            `json:"settled_at"`
}

type SettlementCoordinator struct {
	ledger    *Ledger
	debt      *DebtEngine
	operators *OperatorRegistry
	batches   map[string]*SettlementBatch
}

func NewSettlementCoordinator(ledger *Ledger, debt *DebtEngine, operators *OperatorRegistry) *SettlementCoordinator {
	return &SettlementCoordinator{
		ledger:    ledger,
		debt:      debt,
		operators: operators,
		batches:   make(map[string]*SettlementBatch),
	}
}

func (c *SettlementCoordinator) CreateBatch(id string, operatorID string, calls ...SettlementCall) (*SettlementBatch, error) {
	if _, ok := c.batches[id]; ok {
		return nil, fmt.Errorf("batch %s exists", id)
	}
	total := Amount(0)
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = StableID("call", id, AmountToString(Amount(i)), string(calls[i].Action), calls[i].ProductID)
		}
		if calls[i].Ref == "" {
			calls[i].Ref = id + ":" + calls[i].ID
		}
		total += Positive(calls[i].Amount)
	}
	batch := &SettlementBatch{
		ID:          id,
		OperatorID:  operatorID,
		Calls:       append([]SettlementCall(nil), calls...),
		Status:      "created",
		TotalAmount: total,
		CreatedAt:   c.ledger.Clock(),
	}
	c.batches[id] = batch
	c.ledger.Emit("settlement_batch_created", ProductSystem, id, operatorID, "", total, "info", "settlement batch created", map[string]interface{}{
		"calls": len(calls),
	})
	return batch, nil
}

func (c *SettlementCoordinator) Execute(id string) error {
	batch, ok := c.batches[id]
	if !ok {
		return fmt.Errorf("unknown batch %s", id)
	}
	if batch.Status != "created" {
		return fmt.Errorf("batch %s not executable", id)
	}
	if err := c.operators.Require(batch.OperatorID, ProductSystem, batch.TotalAmount, id+":operator"); err != nil {
		batch.Status = "failed"
		batch.Failure = err.Error()
		return err
	}
	for _, call := range batch.Calls {
		if err := c.executeCall(call); err != nil {
			batch.Status = "failed"
			batch.Failure = err.Error()
			c.ledger.Emit("settlement_batch_failed", ProductSystem, id, call.Account, call.Asset, call.Amount, "warning", err.Error(), map[string]interface{}{
				"call": call.ID,
			})
			return err
		}
	}
	batch.Status = "settled"
	batch.SettledAt = c.ledger.Clock()
	c.ledger.Emit("settlement_batch_settled", ProductSystem, id, batch.OperatorID, "", batch.TotalAmount, "info", "settlement batch settled", map[string]interface{}{
		"calls": len(batch.Calls),
	})
	return nil
}

func (c *SettlementCoordinator) executeCall(call SettlementCall) error {
	switch call.Action {
	case ActionMove:
		return c.ledger.Move(call.Product, call.ProductID, call.From, call.To, call.Asset, call.Amount, call.Ref, call.Memo)
	case ActionDebtOpen:
		_, err := c.debt.OpenObligation(call.Product, call.ProductID, call.Account, call.Asset, call.Amount, call.Amount, call.Ref)
		return err
	case ActionDebtCredit:
		return c.debt.ApplyProductCredit(call.Product, call.ProductID, call.Account, call.Asset, call.Amount, call.Ref)
	case ActionEmit:
		c.ledger.Emit("settlement_call_observed", call.Product, call.ProductID, call.Account, call.Asset, call.Amount, "info", call.Memo, map[string]interface{}{
			"call": call.ID,
		})
		return nil
	default:
		return fmt.Errorf("unknown settlement action %s", call.Action)
	}
}

func (c *SettlementCoordinator) Batches() []SettlementBatch {
	keys := make([]string, 0, len(c.batches))
	for key := range c.batches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]SettlementBatch, 0, len(keys))
	for _, key := range keys {
		batch := *c.batches[key]
		batch.Calls = append([]SettlementCall(nil), batch.Calls...)
		out = append(out, batch)
	}
	return out
}

func (c *SettlementCoordinator) PendingAmount() Amount {
	total := Amount(0)
	for _, batch := range c.batches {
		if batch.Status == "created" {
			total += batch.TotalAmount
		}
	}
	return total
}

func (c *SettlementCoordinator) FailedBatches() []SettlementBatch {
	out := make([]SettlementBatch, 0)
	for _, batch := range c.batches {
		if batch.Status == "failed" {
			item := *batch
			item.Calls = append([]SettlementCall(nil), batch.Calls...)
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func NewMoveCall(product ProductKind, productID string, from string, to string, asset string, amount Amount, memo string) SettlementCall {
	return SettlementCall{
		ID:        StableID("call", string(product), productID, from, to, asset, AmountToString(amount)),
		Action:    ActionMove,
		Product:   product,
		ProductID: productID,
		From:      from,
		To:        to,
		Account:   from,
		Asset:     asset,
		Amount:    amount,
		Memo:      memo,
	}
}

func NewDebtOpenCall(product ProductKind, productID string, account string, asset string, amount Amount) SettlementCall {
	return SettlementCall{
		ID:        StableID("call", "open", string(product), productID, account, asset, AmountToString(amount)),
		Action:    ActionDebtOpen,
		Product:   product,
		ProductID: productID,
		Account:   account,
		Asset:     asset,
		Amount:    amount,
		Memo:      "open local and global debt",
	}
}

func NewDebtCreditCall(product ProductKind, productID string, account string, asset string, amount Amount) SettlementCall {
	return SettlementCall{
		ID:        StableID("call", "credit", string(product), productID, account, asset, AmountToString(amount)),
		Action:    ActionDebtCredit,
		Product:   product,
		ProductID: productID,
		Account:   account,
		Asset:     asset,
		Amount:    amount,
		Memo:      "credit shared debt",
	}
}

func BatchDigest(batch SettlementBatch) string {
	parts := []string{
		batch.ID,
		batch.OperatorID,
		batch.Status,
		AmountToString(batch.TotalAmount),
	}
	for _, call := range batch.Calls {
		parts = append(parts, fmt.Sprintf("%s:%s:%s:%s:%d", call.ID, call.Action, call.Product, call.Asset, call.Amount))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

