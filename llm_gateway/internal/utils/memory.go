package utils

import (
	"math"
)

const redisOverhead = 3.0 // 60% for Redis FLAT index x 25% for overlapping pages x 50% for metadata

// normalPDF is the standard normal PDF: mean = 0, std dev = 1
func normalPDF(x float64) float64 {
	return 1.0 / math.Sqrt(2.0*math.Pi) * math.Exp(-0.5*x*x)
}

// estimateMemory uses 6 trapezoids from -3σ to +3σ
// to approximate the total bytes needed for N pages.
func EstimateMemory(
	numPages int, // total number of pages
	meanBytes int, // average page size in bytes (μ)
	stdDevBytes int, // standard deviation in bytes (σ)
) uint64 {
	if numPages <= 0 || meanBytes < 0 || stdDevBytes < 0 {
		// Return 0 or handle invalid arguments as needed
		return 0
	}

	n := float64(numPages)
	mu := float64(meanBytes)
	sigma := float64(stdDevBytes)

	// Breakpoints at x = -3, -2, -1, 0, 1, 2, 3 (standard deviations)
	xs := []float64{-3, -2, -1, 0, 1, 2, 3}
	// Evaluate standard normal PDF at each breakpoint
	pdfVals := make([]float64, len(xs))
	for i := range xs {
		pdfVals[i] = normalPDF(xs[i])
	}

	// We'll do 6 trapezoids between these 7 points
	var totalFrac float64
	var totalMem float64

	for i := 0; i < 6; i++ {
		x0 := xs[i]
		x1 := xs[i+1]

		// fraction of the distribution in [x0, x1], approximate via trapezoid rule
		frac := 0.5 * (pdfVals[i] + pdfVals[i+1]) * (x1 - x0)
		// midpoint for page size
		mid := 0.5 * (x0 + x1)
		// average size in this interval: μ + mid * σ
		avgSize := mu + mid*sigma
		if avgSize < 0 {
			avgSize = 0 // can't have negative size
		}

		// fraction * N = how many pages land in [x0, x1]
		// multiply by average size to get total bytes in that sub-interval
		mem := frac * n * avgSize
		totalMem += mem
		totalFrac += frac
	}

	// Integral of the standard normal from -3..3 is ~0.9973
	// This scaling ensures we effectively account for "almost all" pages in ±3σ.
	const integralMinus3To3 = 0.9973
	scale := integralMinus3To3 / totalFrac

	// Add Redis overhead and return total memory needed
	return uint64(redisOverhead * totalMem * scale)
}
