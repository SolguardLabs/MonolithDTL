package main

import (
	"fmt"
	"sort"
)

type RouteBook struct {
	ledger    *Ledger
	debt      *DebtEngine
	operators *OperatorRegistry
	routes    map[string]*RoutePlan
}

func NewRouteBook(ledger *Ledger, debt *DebtEngine, operators *OperatorRegistry) *RouteBook {
	return &RouteBook{
		ledger:    ledger,
		debt:      debt,
		operators: operators,
		routes:    make(map[string]*RoutePlan),
	}
}

func (b *RouteBook) Create(id string, source string, destination string, asset string, notional Amount, minOut Amount, slippageBps int, operatorID string) (*RoutePlan, error) {
	if _, ok := b.routes[id]; ok {
		return nil, fmt.Errorf("route %s exists", id)
	}
	if notional <= 0 {
		return nil, fmt.Errorf("route notional must be positive")
	}
	if err := b.operators.Require(operatorID, ProductRoute, notional, id+":create"); err != nil {
		return nil, err
	}
	plan := &RoutePlan{
		ID:            id,
		SourceAccount: source,
		Destination:   destination,
		Asset:         asset,
		Notional:      notional,
		MinOut:        minOut,
		SlippageBps:   slippageBps,
		OperatorID:    operatorID,
		Status:        "created",
		CreatedAt:     b.ledger.Clock(),
	}
	b.routes[id] = plan
	b.ledger.Emit("route_created", ProductRoute, id, source, asset, notional, "info", "route created", map[string]interface{}{
		"destination": destination,
		"operator":    operatorID,
	})
	return plan, nil
}

func (b *RouteBook) AddLeg(routeID string, from string, to string, asset string, amount Amount, product ProductKind, productID string) error {
	plan, ok := b.routes[routeID]
	if !ok {
		return fmt.Errorf("unknown route %s", routeID)
	}
	if plan.Status != "created" && plan.Status != "planned" {
		return fmt.Errorf("route %s cannot add legs in status %s", routeID, plan.Status)
	}
	leg := RouteLeg{
		ID:          StableID("leg", routeID, from, to, asset, AmountToString(amount), productID),
		FromAccount: from,
		ToAccount:   to,
		Asset:       asset,
		Amount:      amount,
		Product:     product,
		ProductID:   productID,
		OperatorID:  plan.OperatorID,
		Status:      "planned",
	}
	plan.Legs = append(plan.Legs, leg)
	plan.Status = "planned"
	b.ledger.Emit("route_leg_added", ProductRoute, routeID, from, asset, amount, "info", "route leg added", map[string]interface{}{
		"to":      to,
		"product": string(product),
	})
	return nil
}

func (b *RouteBook) Settle(routeID string, ref string) error {
	plan, ok := b.routes[routeID]
	if !ok {
		return fmt.Errorf("unknown route %s", routeID)
	}
	if plan.Status != "planned" && plan.Status != "created" {
		return fmt.Errorf("route %s not settleable", routeID)
	}
	gross := Amount(0)
	for _, leg := range plan.Legs {
		gross += leg.Amount
	}
	if gross == 0 {
		gross = plan.Notional
		if err := b.AddLeg(routeID, plan.SourceAccount, plan.Destination, plan.Asset, gross, ProductRoute, routeID); err != nil {
			return err
		}
	}
	out := SubBps(gross, plan.SlippageBps)
	if out < plan.MinOut {
		plan.Status = "rejected"
		plan.RejectedReason = "min_out"
		b.ledger.Emit("route_rejected", ProductRoute, routeID, plan.SourceAccount, plan.Asset, out, "warning", "route rejected by min_out", map[string]interface{}{
			"min_out": plan.MinOut,
		})
		return fmt.Errorf("route min out not met")
	}
	if err := b.operators.Require(plan.OperatorID, ProductRoute, gross, ref); err != nil {
		return err
	}
	for i := range plan.Legs {
		leg := &plan.Legs[i]
		if err := b.ledger.Move(ProductRoute, routeID, leg.FromAccount, leg.ToAccount, leg.Asset, leg.Amount, ref+":"+leg.ID, "route leg settle"); err != nil {
			leg.Status = "failed"
			plan.Status = "rejected"
			plan.RejectedReason = err.Error()
			return err
		}
		leg.Status = "settled"
		plan.SettledAmount += leg.Amount
	}
	plan.Status = "settled"
	plan.SettledAt = b.ledger.Clock()
	if _, err := b.debt.OpenObligation(ProductRoute, routeID, plan.SourceAccount, plan.Asset, MulBps(gross, 500), gross, ref+":route-risk"); err != nil {
		return err
	}
	if err := b.debt.ApplyProductCredit(ProductRoute, routeID, plan.SourceAccount, plan.Asset, MulBps(gross, 500), ref+":route-risk-close"); err != nil {
		return err
	}
	b.ledger.Emit("route_settled", ProductRoute, routeID, plan.SourceAccount, plan.Asset, plan.SettledAmount, "info", "route settled", map[string]interface{}{
		"legs": len(plan.Legs),
	})
	return nil
}

func (b *RouteBook) Routes() []RoutePlan {
	keys := make([]string, 0, len(b.routes))
	for key := range b.routes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]RoutePlan, 0, len(keys))
	for _, key := range keys {
		plan := *b.routes[key]
		plan.Legs = append([]RouteLeg(nil), b.routes[key].Legs...)
		out = append(out, plan)
	}
	return out
}

func (b *RouteBook) ExecutedCount() int {
	count := 0
	for _, route := range b.routes {
		if route.Status == "settled" {
			count++
		}
	}
	return count
}

func (b *RouteBook) RoutesConserved() bool {
	for _, route := range b.routes {
		if route.Status != "settled" {
			continue
		}
		total := Amount(0)
		for _, leg := range route.Legs {
			total += leg.Amount
			if leg.Status != "settled" {
				return false
			}
		}
		if total != route.SettledAmount {
			return false
		}
	}
	return true
}

func (b *RouteBook) ExposureByAccount(account string, asset string) Amount {
	total := Amount(0)
	for _, route := range b.routes {
		if route.SourceAccount == account && route.Asset == asset && route.Status != "settled" {
			total += route.Notional
		}
	}
	return total
}
