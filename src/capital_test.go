package main

import (
	"testing"
)

func TestDefaultCapitalPolicy(t *testing.T) {
	policy := DefaultCapitalPolicy()
	if policy.MinCapitalRatioBps <= policy.HaltCapitalRatioBps {
		t.Fatal("minimum capital ratio must exceed halt ratio")
	}
	if policy.ConcentrationThresholdPpm <= 0 || policy.ConcentrationThresholdPpm >= PpmDenominator {
		t.Fatal("invalid concentration threshold")
	}
}

func TestExpectedLossUsesProbabilityAndRecovery(t *testing.T) {
	assessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("credit", 1_000_000_000, "credit"),
	}, 1_000_000_000)
	if assessment.ExpectedLoss != 5_000_000 {
		t.Fatalf("unexpected expected loss: %d", assessment.ExpectedLoss)
	}
}

func TestLongerHorizonRaisesMarketStress(t *testing.T) {
	short := testExposure("short", 1_000_000_000, "credit")
	long := short
	long.ID = "long"
	long.HorizonDays = 9
	shortAssessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{short}, 1_000_000_000)
	longAssessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{long}, 1_000_000_000)
	if longAssessment.MarketStress <= shortAssessment.MarketStress {
		t.Fatal("longer horizon must increase market stress")
	}
}

func TestLowerLiquidityRaisesCharge(t *testing.T) {
	liquid := testExposure("liquid", 1_000_000_000, "credit")
	liquid.LiquidityBps = 9_900
	illiquid := liquid
	illiquid.ID = "illiquid"
	illiquid.LiquidityBps = 4_000
	left := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{liquid}, 1_000_000_000)
	right := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{illiquid}, 1_000_000_000)
	if right.LiquidityCharge <= left.LiquidityCharge {
		t.Fatal("lower liquidity must increase the liquidity charge")
	}
}

func TestConcentrationHhi(t *testing.T) {
	tests := []struct {
		name      string
		exposures []RiskExposure
		want      int
	}{
		{
			name:      "single cluster",
			exposures: []RiskExposure{testExposure("a", 1_000_000_000, "one")},
			want:      1_000_000,
		},
		{
			name: "two balanced clusters",
			exposures: []RiskExposure{
				testExposure("a", 500_000_000, "one"),
				testExposure("b", 500_000_000, "two"),
			},
			want: 500_000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assessment := EvaluateCapital(DefaultCapitalPolicy(), tt.exposures, 2_000_000_000)
			if assessment.ConcentrationHhiPpm != tt.want {
				t.Fatalf("unexpected HHI: got %d want %d", assessment.ConcentrationHhiPpm, tt.want)
			}
		})
	}
}

func TestDiversificationReducesConcentrationCharge(t *testing.T) {
	concentrated := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("a", 1_000_000_000, "one"),
	}, 2_000_000_000)
	diversified := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("a", 250_000_000, "one"),
		testExposure("b", 250_000_000, "two"),
		testExposure("c", 250_000_000, "three"),
		testExposure("d", 250_000_000, "four"),
	}, 2_000_000_000)
	if diversified.ConcentrationCharge >= concentrated.ConcentrationCharge {
		t.Fatal("diversification must reduce the concentration charge")
	}
}

func TestCapitalDecisionBoundaries(t *testing.T) {
	policy := DefaultCapitalPolicy()
	tests := []struct {
		name      string
		capital   int
		liquidity int
		want      CapitalDecision
	}{
		{"open", 15_000, 11_000, CapitalOpen},
		{"capital restricted", 10_000, 11_000, CapitalRestricted},
		{"liquidity restricted", 15_000, 8_500, CapitalRestricted},
		{"halted", 7_000, 11_000, CapitalHalted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := capitalDecision(policy, tt.capital, tt.liquidity); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
}

func TestZeroCapitalHalts(t *testing.T) {
	assessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("a", 1_000_000_000, "credit"),
	}, 0)
	if assessment.Decision != CapitalHalted {
		t.Fatalf("unexpected decision: %s", assessment.Decision)
	}
}

func TestNonPositiveExposureIsIgnored(t *testing.T) {
	assessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("zero", 0, "credit"),
		testExposure("negative", -1, "credit"),
	}, 1_000_000_000)
	if assessment.GrossExposure != 0 || len(assessment.Charges) != 0 {
		t.Fatal("non-positive exposure must be ignored")
	}
}

func TestExposureNormalization(t *testing.T) {
	exposure := testExposure("normalized", 100_000_000, "")
	exposure.ProbabilityBps = 50_000
	exposure.RecoveryBps = -1
	exposure.VolatilityBps = 80_000
	exposure.LiquidityBps = 70_000
	exposure.HorizonDays = 0
	assessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{exposure}, 1_000_000_000)
	if len(assessment.Concentration) != 1 || assessment.Concentration[0].Cluster != "unclassified" {
		t.Fatal("empty cluster must be normalized")
	}
	if assessment.ExpectedLoss != 100_000_000 {
		t.Fatalf("probability and recovery were not clamped: %d", assessment.ExpectedLoss)
	}
}

func TestAssessmentIsOrderIndependent(t *testing.T) {
	exposures := []RiskExposure{
		testExposure("b", 600_000_000, "two"),
		testExposure("a", 400_000_000, "one"),
	}
	left := EvaluateCapital(DefaultCapitalPolicy(), exposures, 1_000_000_000)
	exposures[0], exposures[1] = exposures[1], exposures[0]
	right := EvaluateCapital(DefaultCapitalPolicy(), exposures, 1_000_000_000)
	if left.Digest != right.Digest {
		t.Fatalf("assessment digest depends on input order: %s != %s", left.Digest, right.Digest)
	}
}

func TestStressLadderIsMonotonic(t *testing.T) {
	assessment := EvaluateCapital(DefaultCapitalPolicy(), []RiskExposure{
		testExposure("a", 1_000_000_000, "credit"),
	}, 1_000_000_000)
	if len(assessment.StressLadder) != 4 {
		t.Fatalf("unexpected stress ladder length: %d", len(assessment.StressLadder))
	}
	for i := 1; i < len(assessment.StressLadder); i++ {
		if assessment.StressLadder[i].RequiredCapital <= assessment.StressLadder[i-1].RequiredCapital {
			t.Fatal("required capital must increase along the ladder")
		}
		if assessment.StressLadder[i].CapitalRatioBps > assessment.StressLadder[i-1].CapitalRatioBps {
			t.Fatal("capital ratio must not improve under a larger shock")
		}
	}
}

func TestIntegerSquareRoot(t *testing.T) {
	tests := map[int]int{0: 0, 1: 1, 2: 1, 4: 2, 15: 3, 16: 4, 17: 4, 100_000_000: 10_000}
	for input, want := range tests {
		if got := intSqrt(input); got != want {
			t.Fatalf("sqrt(%d): got %d want %d", input, got, want)
		}
	}
}

func TestBuildCapitalAssessmentFromProtocolState(t *testing.T) {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	assessment := BuildCapitalAssessment(ctx)
	if assessment.GrossExposure <= 0 || assessment.AvailableCapital <= 0 {
		t.Fatal("protocol assessment must include exposure and available capital")
	}
	if assessment.Digest == "" || len(assessment.Charges) < 2 {
		t.Fatal("protocol assessment lacks deterministic detail")
	}
}

func TestScenarioReportCommitsCapitalDigest(t *testing.T) {
	ctx := NewProtocolContext()
	runCommonBootstrap(ctx)
	report := BuildReport(ctx, "test")
	if report.Capital.Digest == "" || report.StateDigest == "" {
		t.Fatal("report digests must be present")
	}
	changed := report
	changed.Capital.Digest = "changed"
	if reportDigest(changed) == report.StateDigest {
		t.Fatal("state digest must commit to the capital assessment")
	}
}

func testExposure(id string, notional Amount, cluster string) RiskExposure {
	return RiskExposure{
		ID:                id,
		Product:           ProductVault,
		Asset:             "mUSD",
		Cluster:           cluster,
		Notional:          notional,
		ProbabilityBps:    100,
		RecoveryBps:       5_000,
		VolatilityBps:     500,
		LiquidityBps:      10_000,
		HorizonDays:       1,
		CorrelationBucket: "usd-credit",
	}
}
