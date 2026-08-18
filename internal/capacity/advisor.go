package capacity

import (
	"fmt"
	"math"
)

// TierConfig represents a tier in a multi-tier Bloom filter deployment.
// Each tier has different capacity and FPR characteristics.
type TierConfig struct {
	Name      string
	ExpectedN uint
	TargetFPR float64
}

// TierPlan is a recommended configuration for a single tier.
type TierPlan struct {
	Config   TierConfig
	Params   Params
	MemoryMB float64 // estimated memory usage in MB
}

// MultiTierPlan plans a multi-tier Bloom filter system where traffic flows
// through tiers in order (e.g., hot → warm → cold).
func MultiTierPlan(tiers []TierConfig) ([]TierPlan, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("capacity: no tiers specified")
	}
	plans := make([]TierPlan, len(tiers))
	for i, tc := range tiers {
		if tc.ExpectedN == 0 {
			return nil, ErrInvalidCount
		}
		if tc.TargetFPR <= 0 || tc.TargetFPR >= 1 {
			return nil, ErrInvalidFPR
		}
		params, err := Recommend(tc.ExpectedN, tc.TargetFPR)
		if err != nil {
			return nil, err
		}
		memMB := float64(params.Bytes) / (1024.0 * 1024.0)
		plans[i] = TierPlan{
			Config:   tc,
			Params:   params,
			MemoryMB: memMB,
		}
	}
	return plans, nil
}

// CombinedFPR computes the combined false positive rate for a cascaded
// system of filters: FPR_combined = product of individual FPRs.
// This applies when all filters must agree (AND logic).
func CombinedFPR(rates []float64) float64 {
	product := 1.0
	for _, r := range rates {
		product *= r
	}
	return product
}

// ParallelFPR computes the FPR for a system where any filter saying "present"
// is treated as present (OR logic): FPR = 1 - product(1 - fpr_i).
func ParallelFPR(rates []float64) float64 {
	product := 1.0
	for _, r := range rates {
		product *= (1 - r)
	}
	return 1 - product
}

// MemoryBudgetPlan finds the best (m, k) parameters that fit within a given
// memory budget in bytes for the expected number of items.
// Returns the achievable FPR within the budget.
func MemoryBudgetPlan(budgetBytes uint, expectedN uint) (Params, error) {
	if budgetBytes == 0 || expectedN == 0 {
		return Params{}, ErrInvalidCount
	}
	m := budgetBytes * 8 // convert bytes to bits
	k := optimalKForM(m, expectedN)
	actualFPR := computeFPR(m, k, expectedN)

	return Params{
		M:           m,
		K:           k,
		Bytes:       budgetBytes,
		ExpectedN:   expectedN,
		TargetFPR:   actualFPR,
		ActualFPR:   actualFPR,
		BitsPerItem: float64(m) / float64(expectedN),
	}, nil
}

// GrowthProjection estimates filter parameters needed at various growth stages.
// Given current N and growth rate, projects needs for 2x, 5x, and 10x scale.
type GrowthProjection struct {
	CurrentN   uint
	Multiplier uint
	ProjectedN uint
	Params     Params
	MemoryMB   float64
}

// ProjectGrowth generates projections for a filter at multiples of current load.
func ProjectGrowth(currentN uint, targetFPR float64, multipliers []uint) ([]GrowthProjection, error) {
	if currentN == 0 {
		return nil, ErrInvalidCount
	}
	if targetFPR <= 0 || targetFPR >= 1 {
		return nil, ErrInvalidFPR
	}
	projections := make([]GrowthProjection, len(multipliers))
	for i, mult := range multipliers {
		projected := currentN * mult
		params, err := Recommend(projected, targetFPR)
		if err != nil {
			return nil, err
		}
		projections[i] = GrowthProjection{
			CurrentN:   currentN,
			Multiplier: mult,
			ProjectedN: projected,
			Params:     params,
			MemoryMB:   float64(params.Bytes) / (1024.0 * 1024.0),
		}
	}
	return projections, nil
}

// BitsPerItemForFPR returns the theoretical minimum bits per item needed
// to achieve a given FPR: bpi = -log2(fpr) / ln(2) ≈ -1.44 * log2(fpr).
func BitsPerItemForFPR(targetFPR float64) float64 {
	if targetFPR <= 0 || targetFPR >= 1 {
		return 0
	}
	return -math.Log(targetFPR) / (math.Ln2 * math.Ln2)
}

// optimalKForM computes the optimal k for given m and n.
func optimalKForM(m, n uint) uint {
	if n == 0 || m == 0 {
		return 1
	}
	k := float64(m) / float64(n) * math.Ln2
	ki := uint(math.Round(k))
	if ki < 1 {
		ki = 1
	}
	return ki
}

// computeFPR returns the theoretical FPR for given parameters.
func computeFPR(m, k, n uint) float64 {
	if m == 0 {
		return 1
	}
	exponent := -float64(k) * float64(n) / float64(m)
	return math.Pow(1-math.Exp(exponent), float64(k))
}
