package models

import (
	"fmt"
	"math"
)

// NanoUSD is an exact USD amount in billionths of a dollar. All persisted and
// accumulated currency uses this representation; floating point is only used
// at compatibility boundaries such as the existing JSON API and Prometheus.
type NanoUSD int64

const NanoUSDPerUSD NanoUSD = 1_000_000_000

func NanoUSDFromFloat64(amount float64) (NanoUSD, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > float64(math.MaxInt64)/float64(NanoUSDPerUSD) {
		return 0, fmt.Errorf("invalid USD amount %v", amount)
	}
	return NanoUSD(math.Round(amount * float64(NanoUSDPerUSD))), nil
}

func MustNanoUSD(amount float64) NanoUSD {
	value, err := NanoUSDFromFloat64(amount)
	if err != nil {
		panic(err)
	}
	return value
}

func (amount NanoUSD) Float64() float64 {
	return float64(amount) / float64(NanoUSDPerUSD)
}

func (amount NanoUSD) String() string { return fmt.Sprintf("%d", amount) }
