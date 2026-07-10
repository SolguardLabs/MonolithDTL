package main

import "sort"

type RedemptionQueue struct {
	ledger      *Ledger
	vaults      *VaultBook
	redemptions map[string]*RedemptionRequest
}

func NewRedemptionQueue(ledger *Ledger, vaults *VaultBook) *RedemptionQueue {
	return &RedemptionQueue{
		ledger:      ledger,
		vaults:      vaults,
		redemptions: make(map[string]*RedemptionRequest),
	}
}

func (q *RedemptionQueue) RequestVault(vaultID string, account string, shares Amount, minOut Amount, ref string) RedemptionRequest {
	request, err := q.vaults.Redeem(vaultID, account, shares, minOut, ref)
	if err != nil && request.ID == "" {
		request = RedemptionRequest{
			ID:        StableID("red", vaultID, account, AmountToString(shares), ref),
			Product:   ProductVault,
			ProductID: vaultID,
			Account:   account,
			Shares:    shares,
			Status:    "rejected",
			Reason:    err.Error(),
			CreatedAt: q.ledger.Clock(),
		}
	}
	if request.ID == "" {
		request.ID = StableID("red", vaultID, account, AmountToString(shares), ref)
	}
	q.redemptions[request.ID] = &request
	q.ledger.Emit("redemption_recorded", ProductRedemption, request.ID, account, request.Asset, request.ExpectedOut, "info", "redemption recorded", map[string]interface{}{
		"status": request.Status,
		"reason": request.Reason,
	})
	return request
}

func (q *RedemptionQueue) Redemptions() []RedemptionRequest {
	keys := make([]string, 0, len(q.redemptions))
	for key := range q.redemptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]RedemptionRequest, 0, len(keys))
	for _, key := range keys {
		out = append(out, *q.redemptions[key])
	}
	return out
}

func (q *RedemptionQueue) ProcessedCount() int {
	count := 0
	for _, redemption := range q.redemptions {
		if redemption.Status == "processed" {
			count++
		}
	}
	return count
}

func (q *RedemptionQueue) RejectedCount() int {
	count := 0
	for _, redemption := range q.redemptions {
		if redemption.Status == "rejected" {
			count++
		}
	}
	return count
}

func (q *RedemptionQueue) ByAccount(account string) []RedemptionRequest {
	out := make([]RedemptionRequest, 0)
	for _, redemption := range q.redemptions {
		if redemption.Account == account {
			out = append(out, *redemption)
		}
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (q *RedemptionQueue) TotalProcessed(asset string) Amount {
	total := Amount(0)
	for _, redemption := range q.redemptions {
		if redemption.Asset == asset && redemption.Status == "processed" {
			total += redemption.ExpectedOut
		}
	}
	return total
}

