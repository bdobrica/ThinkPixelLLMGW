package models

import "testing"

func TestNanoUSDContract(t *testing.T) {
	tests := []struct {
		usd  float64
		nano NanoUSD
	}{
		{0.0000000004, 0},
		{0.0000000005, 1},
		{0.000000001, 1},
		{12.345678901, 12_345_678_901},
	}
	for _, test := range tests {
		got, err := NanoUSDFromFloat64(test.usd)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.nano {
			t.Fatalf("NanoUSDFromFloat64(%v) = %d, want %d", test.usd, got, test.nano)
		}
	}
}

func TestBudgetBoundaryIsExact(t *testing.T) {
	budget := MustNanoUSD(1)
	if MustNanoUSD(1) != budget {
		t.Fatal("equal spending must compare exactly")
	}
	if MustNanoUSD(1.000000001) != budget+1 {
		t.Fatal("one nano-USD over budget must be distinct")
	}
}
