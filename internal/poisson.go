package internal

import "math"

func poissonCDF(k int, lambda float64) float64 {
	if k < 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i <= k; i++ {
		sum += poissonPMF(i, lambda)
	}
	return sum
}

func poissonPMF(k int, lambda float64) float64 {
	if k < 0 {
		return 0
	}
	return math.Exp(float64(k)*math.Log(lambda) - lambda - logFactorial(k))
}

func logFactorial(n int) float64 {
	if n <= 1 {
		return 0
	}
	result := 0.0
	for i := 2; i <= n; i++ {
		result += math.Log(float64(i))
	}
	return result
}
