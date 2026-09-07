package internal

import "math"

// poissonCDF returns P(X <= k) for X ~ Poisson(lambda). Sum the smaller
// tail directly: subtracting a nearly-one CDF loses rare-event probabilities.
func poissonCDF(k int, lambda float64) float64 {
	if lambda < 0 || math.IsNaN(lambda) || math.IsInf(lambda, 0) {
		return math.NaN()
	}
	if k < 0 {
		return 0
	}
	if lambda == 0 {
		return 1
	}
	if float64(k) < lambda {
		return math.Exp(poissonLogLowerTail(k, lambda))
	}
	return -math.Expm1(poissonLogUpperTail(k+1, lambda))
}

func poissonPMF(k int, lambda float64) float64 {
	if lambda < 0 || math.IsNaN(lambda) || math.IsInf(lambda, 0) {
		return math.NaN()
	}
	if k < 0 {
		return 0
	}
	if lambda == 0 {
		if k == 0 {
			return 1
		}
		return 0
	}
	return math.Exp(poissonLogPMF(k, lambda))
}

func poissonLogPMF(k int, lambda float64) float64 {
	factorial, _ := math.Lgamma(float64(k) + 1)
	return float64(k)*math.Log(lambda) - lambda - factorial
}

// The tail recurrences start at their largest term. Keeping that term in
// logarithmic form avoids underflow, even at subnormal confidence levels.
func poissonLogLowerTail(k int, lambda float64) float64 {
	term, sum := 1.0, 1.0
	for i := k; i > 0; i-- {
		term *= float64(i) / lambda
		next := sum + term
		if next == sum {
			break
		}
		sum = next
	}
	return poissonLogPMF(k, lambda) + math.Log(sum)
}

func poissonLogUpperTail(k int, lambda float64) float64 {
	term, sum := 1.0, 1.0
	for i := k + 1; ; i++ {
		term *= lambda / float64(i)
		next := sum + term
		if next == sum {
			break
		}
		sum = next
	}
	return poissonLogPMF(k, lambda) + math.Log(sum)
}

// poissonBlocks returns the largest n for which P(X >= n) >= probability.
// Callers handle confidence endpoints separately; lambda must be positive and
// finite and probability must lie strictly between zero and one.
func poissonBlocks(lambda, probability float64) int {
	// Normalize first so the logarithm also works for subnormal confidences.
	fraction, exponent := math.Frexp(probability)
	logProbability := math.Log(fraction) + float64(exponent)*math.Ln2
	atLeast := func(n int) bool {
		if n == 0 {
			return true
		}
		if float64(n) > lambda {
			return poissonLogUpperTail(n, lambda) >= logProbability
		}
		return poissonCDF(n-1, lambda) <= 1-probability
	}

	// A fixed multiple of the mean truncates low-confidence estimates. Bracket
	// the quantile first, then search without allocating a probability table.
	lo, hi := 0, max(1, int(math.Ceil(lambda)))
	for atLeast(hi) {
		lo = hi
		hi *= 2
	}
	for lo+1 < hi {
		mid := lo + (hi-lo)/2
		if atLeast(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}
