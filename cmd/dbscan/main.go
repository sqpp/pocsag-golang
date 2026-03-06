package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"sort"

	pocsag "github.com/sqpp/pocsag-golang/v2"
)

func main() {
	packet := pocsag.CreatePOCSAGPacketWithBaudRate(1234560, "TEST MESSAGE", pocsag.FuncAlphanumeric, pocsag.BaudRate1200)
	iqSamples := pocsag.GenerateFSKSamples(packet, pocsag.BaudRate1200)
	cfg := pocsag.DefaultWaterfallConfig()

	numComplexSamples := len(iqSamples) / 2
	complexSamples := make([]complex128, numComplexSamples)
	for i := 0; i < numComplexSamples; i++ {
		complexSamples[i] = complex(float64(iqSamples[i*2])/32768.0, float64(iqSamples[i*2+1])/32768.0)
	}

	stepSize := int(float64(cfg.FFTSize) * (1.0 - cfg.Overlap))
	if stepSize < 1 {
		stepSize = 1
	}

	all := []float64{}
	numWindows := (numComplexSamples - cfg.FFTSize) / stepSize
	for windowIdx := 0; windowIdx < numWindows; windowIdx++ {
		startIdx := windowIdx * stepSize
		window := make([]complex128, cfg.FFTSize)
		for i := 0; i < cfg.FFTSize; i++ {
			hannWeight := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(cfg.FFTSize-1)))
			window[i] = complexSamples[startIdx+i] * complex(hannWeight, 0)
		}

		coeffs := pocsag.ComplexFFT(window)
		for i := range coeffs {
			coeffs[i] /= complex(float64(cfg.FFTSize), 0)
		}

		shifted := make([]complex128, cfg.FFTSize)
		half := cfg.FFTSize / 2
		for i := 0; i < cfg.FFTSize; i++ {
			shifted[i] = coeffs[(i+half)%cfg.FFTSize]
		}

		for _, c := range shifted {
			mag := cmplx.Abs(c)
			power := mag * mag
			db := 10.0 * math.Log10(power+1e-12)
			all = append(all, db)
		}
	}

	sort.Float64s(all)
	n := len(all)
	percentile := func(p float64) float64 {
		idx := int(p * float64(n))
		if idx >= n { idx = n - 1 }
		return all[idx]
	}

	fmt.Printf("Power distribution (dB):\n")
	fmt.Printf("  Min (p0):   %.2f dB\n", all[0])
	fmt.Printf("  p10:        %.2f dB\n", percentile(0.10))
	fmt.Printf("  p25:        %.2f dB\n", percentile(0.25))
	fmt.Printf("  p50:        %.2f dB\n", percentile(0.50))
	fmt.Printf("  p75:        %.2f dB\n", percentile(0.75))
	fmt.Printf("  p90:        %.2f dB\n", percentile(0.90))
	fmt.Printf("  p99:        %.2f dB\n", percentile(0.99))
	fmt.Printf("  Max (p100): %.2f dB\n", all[n-1])
}
