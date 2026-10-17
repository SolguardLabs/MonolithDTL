package main

import (
	"fmt"
	"sort"
)

const PpmDenominator = 1_000_000

type CapitalDecision string

const (
	CapitalOpen       CapitalDecision = "open"
	CapitalRestricted CapitalDecision = "restricted"
	CapitalHalted     CapitalDecision = "halted"
)

type RiskExposure struct {
	ID                string      `json:"id"`
	Product           ProductKind `json:"product"`
	Asset             string      `json:"asset"`
	Cluster           string      `json:"cluster"`
	Notional          Amount      `json:"notional"`
	ProbabilityBps    int         `json:"probability_bps"`
	RecoveryBps       int         `json:"recovery_bps"`
	VolatilityBps     int         `json:"volatility_bps"`
	LiquidityBps      int         `json:"liquidity_bps"`
	HorizonDays       int         `json:"horizon_days"`
	CorrelationBucket string      `json:"correlation_bucket"`
}

type CapitalPolicy struct {
	StressMultiplierBps       int `json:"stress_multiplier_bps"`
	LiquidityBpsPerDay        int `json:"liquidity_bps_per_day"`
	MaxLiquidityChargeBps     int `json:"max_liquidity_charge_bps"`
	ConcentrationThresholdPpm int `json:"concentration_threshold_ppm"`
	ConcentrationChargeBps    int `json:"concentration_charge_bps"`
	OperationalRiskBps        int `json:"operational_risk_bps"`
	MinCapitalRatioBps        int `json:"min_capital_ratio_bps"`
	HaltCapitalRatioBps       int `json:"halt_capital_ratio_bps"`
	MinLiquidityCoverageBps   int `json:"min_liquidity_coverage_bps"`
}

type ExposureCharge struct {
	ID              string      `json:"id"`
	Product         ProductKind `json:"product"`
	Asset           string      `json:"asset"`
	Cluster         string      `json:"cluster"`
	Notional        Amount      `json:"notional"`
	ExpectedLoss    Amount      `json:"expected_loss"`
	MarketStress    Amount      `json:"market_stress"`
	LiquidityCharge Amount      `json:"liquidity_charge"`
	TotalCharge     Amount      `json:"total_charge"`
}

type ConcentrationLine struct {
	Cluster  string `json:"cluster"`
	Notional Amount `json:"notional"`
	ShareBps int    `json:"share_bps"`
}

type CapitalStressPoint struct {
	Name            string          `json:"name"`
	ShockBps        int             `json:"shock_bps"`
	RequiredCapital Amount          `json:"required_capital"`
	CapitalRatioBps int             `json:"capital_ratio_bps"`
	Decision        CapitalDecision `json:"decision"`
}

type CapitalAssessment struct {
	Policy               CapitalPolicy        `json:"policy"`
	GrossExposure        Amount               `json:"gross_exposure"`
	LiquidExposure       Amount               `json:"liquid_exposure"`
	ExpectedLoss         Amount               `json:"expected_loss"`
	MarketStress         Amount               `json:"market_stress"`
	LiquidityCharge      Amount               `json:"liquidity_charge"`
	ConcentrationCharge  Amount               `json:"concentration_charge"`
	OperationalCharge    Amount               `json:"operational_charge"`
	RequiredCapital      Amount               `json:"required_capital"`
	AvailableCapital     Amount               `json:"available_capital"`
	CapitalRatioBps      int                  `json:"capital_ratio_bps"`
	LiquidityCoverageBps int                  `json:"liquidity_coverage_bps"`
	ConcentrationHhiPpm  int                  `json:"concentration_hhi_ppm"`
	Decision             CapitalDecision      `json:"decision"`
	Charges              []ExposureCharge     `json:"charges"`
	Concentration        []ConcentrationLine  `json:"concentration"`
	StressLadder         []CapitalStressPoint `json:"stress_ladder"`
	Digest               string               `json:"digest"`
}

func DefaultCapitalPolicy() CapitalPolicy {
	return CapitalPolicy{
		StressMultiplierBps:       14_000,
		LiquidityBpsPerDay:        35,
		MaxLiquidityChargeBps:     2_500,
		ConcentrationThresholdPpm: 350_000,
		ConcentrationChargeBps:    3_000,
		OperationalRiskBps:        75,
		MinCapitalRatioBps:        11_000,
		HaltCapitalRatioBps:       7_500,
		MinLiquidityCoverageBps:   9_000,
	}
}

func EvaluateCapital(policy CapitalPolicy, exposures []RiskExposure, availableCapital Amount) CapitalAssessment {
	policy = normalizeCapitalPolicy(policy)
	ordered := append([]RiskExposure(nil), exposures...)
	sort.Slice(ordered, func(i int, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})

	assessment := CapitalAssessment{
		Policy:           policy,
		AvailableCapital: Positive(availableCapital),
		Charges:          make([]ExposureCharge, 0, len(ordered)),
	}
	clusters := make(map[string]Amount)
	shortDated := Amount(0)

	for _, exposure := range ordered {
		exposure = normalizeExposure(exposure)
		if exposure.Notional <= 0 {
			continue
		}
		charge := chargeExposure(policy, exposure)
		assessment.Charges = append(assessment.Charges, charge)
		assessment.GrossExposure = Add(assessment.GrossExposure, exposure.Notional)
		assessment.LiquidExposure = Add(assessment.LiquidExposure, MulBps(exposure.Notional, exposure.LiquidityBps))
		assessment.ExpectedLoss = Add(assessment.ExpectedLoss, charge.ExpectedLoss)
		assessment.MarketStress = Add(assessment.MarketStress, charge.MarketStress)
		assessment.LiquidityCharge = Add(assessment.LiquidityCharge, charge.LiquidityCharge)
		clusters[exposure.Cluster] = Add(clusters[exposure.Cluster], exposure.Notional)
		if exposure.HorizonDays <= 7 {
			shortDated = Add(shortDated, exposure.Notional)
		}
	}

	assessment.Concentration, assessment.ConcentrationHhiPpm = concentrationLines(clusters, assessment.GrossExposure)
	excessHhi := maxInt(assessment.ConcentrationHhiPpm-policy.ConcentrationThresholdPpm, 0)
	concentrationBps := (excessHhi * policy.ConcentrationChargeBps) / PpmDenominator
	assessment.ConcentrationCharge = MulBps(assessment.GrossExposure, concentrationBps)
	assessment.OperationalCharge = MulBps(assessment.GrossExposure, policy.OperationalRiskBps)
	assessment.RequiredCapital = sumAmounts(
		assessment.ExpectedLoss,
		assessment.MarketStress,
		assessment.LiquidityCharge,
		assessment.ConcentrationCharge,
		assessment.OperationalCharge,
	)
	assessment.CapitalRatioBps = RatioBps(assessment.AvailableCapital, MaxAmountOf(assessment.RequiredCapital, 1))
	assessment.LiquidityCoverageBps = RatioBps(assessment.LiquidExposure, MaxAmountOf(shortDated, 1))
	assessment.Decision = capitalDecision(policy, assessment.CapitalRatioBps, assessment.LiquidityCoverageBps)
	assessment.StressLadder = buildStressLadder(policy, assessment.RequiredCapital, assessment.AvailableCapital, assessment.LiquidityCoverageBps)
	assessment.Digest = capitalDigest(assessment)
	return assessment
}

func BuildCapitalAssessment(ctx *ProtocolContext) CapitalAssessment {
	exposures := make([]RiskExposure, 0)
	for _, vault := range ctx.Vaults.Vaults() {
		notional := Add(vault.TotalAssets, vault.OpenDebt)
		utilization := RatioBps(vault.OpenDebt, MaxAmountOf(notional, 1))
		exposures = append(exposures, RiskExposure{
			ID:                "vault:" + vault.ID,
			Product:           ProductVault,
			Asset:             vault.Asset,
			Cluster:           "core-reserve",
			Notional:          notional,
			ProbabilityBps:    90 + utilization/20,
			RecoveryBps:       8_000,
			VolatilityBps:     450 + utilization/8,
			LiquidityBps:      clampInt(9_500-utilization/3, 3_000, 9_500),
			HorizonDays:       3,
			CorrelationBucket: "usd-credit",
		})
	}
	for _, escrow := range ctx.Escrows.Escrows() {
		remaining := Positive(escrow.Amount - escrow.Released - escrow.Refunded)
		if remaining == 0 {
			continue
		}
		exposures = append(exposures, RiskExposure{
			ID:                "escrow:" + escrow.ID,
			Product:           ProductEscrow,
			Asset:             escrow.Asset,
			Cluster:           "escrow-float",
			Notional:          remaining,
			ProbabilityBps:    55,
			RecoveryBps:       9_200,
			VolatilityBps:     180,
			LiquidityBps:      9_800,
			HorizonDays:       1,
			CorrelationBucket: "usd-settlement",
		})
	}
	for _, route := range ctx.Routes.Routes() {
		if route.Notional <= 0 {
			continue
		}
		liquidity := 9_400
		if route.Status != "settled" {
			liquidity = 7_200
		}
		exposures = append(exposures, RiskExposure{
			ID:                "route:" + route.ID,
			Product:           ProductRoute,
			Asset:             route.Asset,
			Cluster:           "routing-float",
			Notional:          route.Notional,
			ProbabilityBps:    45,
			RecoveryBps:       9_500,
			VolatilityBps:     260 + route.SlippageBps,
			LiquidityBps:      liquidity,
			HorizonDays:       1,
			CorrelationBucket: "usd-settlement",
		})
	}

	available := Amount(0)
	for _, accountID := range []string{"insurance", "treasury"} {
		if account, ok := ctx.Ledger.Account(accountID); ok {
			available = Add(available, account.Balances["mUSD"])
		}
	}
	return EvaluateCapital(DefaultCapitalPolicy(), exposures, available)
}

func chargeExposure(policy CapitalPolicy, exposure RiskExposure) ExposureCharge {
	expected := MulBps(MulBps(exposure.Notional, exposure.ProbabilityBps), BpsDenominator-exposure.RecoveryBps)
	baseMarket := MulBps(exposure.Notional, exposure.VolatilityBps)
	horizonFactorBps := intSqrt(exposure.HorizonDays * 100_000_000)
	market := MulBps(MulBps(baseMarket, horizonFactorBps), policy.StressMultiplierBps)
	liquidityGapBps := BpsDenominator - exposure.LiquidityBps
	liquidityBps := minInt(policy.MaxLiquidityChargeBps, liquidityGapBps+exposure.HorizonDays*policy.LiquidityBpsPerDay)
	liquidity := MulBps(exposure.Notional, liquidityBps)
	return ExposureCharge{
		ID:              exposure.ID,
		Product:         exposure.Product,
		Asset:           exposure.Asset,
		Cluster:         exposure.Cluster,
		Notional:        exposure.Notional,
		ExpectedLoss:    expected,
		MarketStress:    market,
		LiquidityCharge: liquidity,
		TotalCharge:     sumAmounts(expected, market, liquidity),
	}
}

func concentrationLines(clusters map[string]Amount, total Amount) ([]ConcentrationLine, int) {
	keys := make([]string, 0, len(clusters))
	for key := range clusters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]ConcentrationLine, 0, len(keys))
	hhiPpm := 0
	for _, key := range keys {
		shareBps := RatioBps(clusters[key], MaxAmountOf(total, 1))
		hhiPpm += (shareBps * shareBps) / 100
		lines = append(lines, ConcentrationLine{Cluster: key, Notional: clusters[key], ShareBps: shareBps})
	}
	return lines, clampInt(hhiPpm, 0, PpmDenominator)
}

func buildStressLadder(policy CapitalPolicy, required Amount, available Amount, liquidityCoverageBps int) []CapitalStressPoint {
	points := []struct {
		name  string
		shock int
	}{
		{"base", 10_000},
		{"adverse", 12_500},
		{"severe", 15_000},
		{"reverse", 20_000},
	}
	out := make([]CapitalStressPoint, 0, len(points))
	for _, point := range points {
		stressed := MulBps(required, point.shock)
		ratio := RatioBps(available, MaxAmountOf(stressed, 1))
		out = append(out, CapitalStressPoint{
			Name:            point.name,
			ShockBps:        point.shock,
			RequiredCapital: stressed,
			CapitalRatioBps: ratio,
			Decision:        capitalDecision(policy, ratio, liquidityCoverageBps),
		})
	}
	return out
}

func capitalDecision(policy CapitalPolicy, capitalRatioBps int, liquidityCoverageBps int) CapitalDecision {
	if capitalRatioBps < policy.HaltCapitalRatioBps {
		return CapitalHalted
	}
	if capitalRatioBps < policy.MinCapitalRatioBps || liquidityCoverageBps < policy.MinLiquidityCoverageBps {
		return CapitalRestricted
	}
	return CapitalOpen
}

func capitalDigest(assessment CapitalAssessment) string {
	parts := []string{
		fmt.Sprintf("gross:%d", assessment.GrossExposure),
		fmt.Sprintf("required:%d", assessment.RequiredCapital),
		fmt.Sprintf("available:%d", assessment.AvailableCapital),
		fmt.Sprintf("ratio:%d", assessment.CapitalRatioBps),
		fmt.Sprintf("liquidity:%d", assessment.LiquidityCoverageBps),
		fmt.Sprintf("hhi:%d", assessment.ConcentrationHhiPpm),
		"decision:" + string(assessment.Decision),
	}
	for _, charge := range assessment.Charges {
		parts = append(parts, fmt.Sprintf("charge:%s:%d:%d", charge.ID, charge.Notional, charge.TotalCharge))
	}
	for _, point := range assessment.StressLadder {
		parts = append(parts, fmt.Sprintf("stress:%s:%d:%s", point.Name, point.RequiredCapital, point.Decision))
	}
	sort.Strings(parts)
	return Digest(parts...)
}

func normalizeCapitalPolicy(policy CapitalPolicy) CapitalPolicy {
	defaults := DefaultCapitalPolicy()
	if policy.StressMultiplierBps <= 0 {
		policy.StressMultiplierBps = defaults.StressMultiplierBps
	}
	if policy.LiquidityBpsPerDay <= 0 {
		policy.LiquidityBpsPerDay = defaults.LiquidityBpsPerDay
	}
	if policy.MaxLiquidityChargeBps <= 0 {
		policy.MaxLiquidityChargeBps = defaults.MaxLiquidityChargeBps
	}
	if policy.ConcentrationThresholdPpm <= 0 {
		policy.ConcentrationThresholdPpm = defaults.ConcentrationThresholdPpm
	}
	if policy.ConcentrationChargeBps <= 0 {
		policy.ConcentrationChargeBps = defaults.ConcentrationChargeBps
	}
	if policy.OperationalRiskBps <= 0 {
		policy.OperationalRiskBps = defaults.OperationalRiskBps
	}
	if policy.MinCapitalRatioBps <= 0 {
		policy.MinCapitalRatioBps = defaults.MinCapitalRatioBps
	}
	if policy.HaltCapitalRatioBps <= 0 {
		policy.HaltCapitalRatioBps = defaults.HaltCapitalRatioBps
	}
	if policy.MinLiquidityCoverageBps <= 0 {
		policy.MinLiquidityCoverageBps = defaults.MinLiquidityCoverageBps
	}
	return policy
}

func normalizeExposure(exposure RiskExposure) RiskExposure {
	exposure.Notional = Positive(exposure.Notional)
	exposure.ProbabilityBps = clampInt(exposure.ProbabilityBps, 0, BpsDenominator)
	exposure.RecoveryBps = clampInt(exposure.RecoveryBps, 0, BpsDenominator)
	exposure.VolatilityBps = clampInt(exposure.VolatilityBps, 0, BpsDenominator)
	exposure.LiquidityBps = clampInt(exposure.LiquidityBps, 0, BpsDenominator)
	if exposure.HorizonDays <= 0 {
		exposure.HorizonDays = 1
	}
	if exposure.Cluster == "" {
		exposure.Cluster = "unclassified"
	}
	return exposure
}

func intSqrt(value int) int {
	if value <= 0 {
		return 0
	}
	left, right := 1, value
	answer := 1
	for left <= right {
		mid := left + (right-left)/2
		if mid <= value/mid {
			answer = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return answer
}

func sumAmounts(values ...Amount) Amount {
	total := Amount(0)
	for _, value := range values {
		total = Add(total, value)
	}
	return total
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func clampInt(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
