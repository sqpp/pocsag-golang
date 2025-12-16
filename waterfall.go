package pocsag

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/cmplx"
)

// WaterfallConfig holds configuration for waterfall generation
type WaterfallConfig struct {
	Width      int     // Width of output image (time axis)
	Height     int     // Height of output image (frequency axis)
	FFTSize    int     // FFT window size
	Overlap    float64 // Overlap between FFT windows (0.0 to 1.0)
	MinFreq    float64 // Minimum frequency to display (Hz)
	MaxFreq    float64 // Maximum frequency to display (Hz)
	SampleRate int     // Audio sample rate
}

// DefaultWaterfallConfig returns sensible defaults for POCSAG FSK waterfall
// Configured to look like an SDR waterfall with RF carrier
func DefaultWaterfallConfig() WaterfallConfig {
	return WaterfallConfig{
		Width:      1200,  // Width for time axis
		Height:     400,   // Height for frequency display
		FFTSize:    4096,  // Large FFT for high frequency resolution (like SDR apps)
		Overlap:    0.95,  // Very high overlap (95%) for smooth SDR-style display
		MinFreq:    0,     // Show full spectrum from 0
		MaxFreq:    24000, // Up to 24 kHz (Nyquist frequency)
		SampleRate: SampleRate,
	}
}

// GenerateWaterfall creates a waterfall (spectrogram) image from IQ samples
// Samples are expected to be interleaved I/Q: [I0, Q0, I1, Q1, ...]
func GenerateWaterfall(samples []int16, config WaterfallConfig) (image.Image, error) {
	// Convert interleaved IQ samples to complex numbers
	numComplexSamples := len(samples) / 2
	complexSamples := make([]complex128, numComplexSamples)
	for i := 0; i < numComplexSamples; i++ {
		// I = real, Q = imaginary
		iSample := float64(samples[i*2]) / 32768.0
		qSample := float64(samples[i*2+1]) / 32768.0
		complexSamples[i] = complex(iSample, qSample)
	}

	// Calculate step size based on overlap
	stepSize := int(float64(config.FFTSize) * (1.0 - config.Overlap))
	numWindows := (numComplexSamples - config.FFTSize) / stepSize

	if numWindows <= 0 {
		numWindows = 1
	}

	// Calculate frequency bins
	freqBinSize := float64(config.SampleRate) / float64(config.FFTSize)
	minBin := int(config.MinFreq / freqBinSize)
	maxBin := int(config.MaxFreq / freqBinSize)
	if maxBin > config.FFTSize {
		maxBin = config.FFTSize
	}
	numBins := maxBin - minBin

	// Create output image
	img := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))

	// Define dB range for display (SDR-style with high contrast)
	const minDB = -80.0 // Noise floor threshold for blue background
	const maxDB = -20.0 // Peak signals (yellow/red)
	const dbRange = maxDB - minDB

	// Process each time window and draw directly
	for windowIdx := 0; windowIdx < numWindows; windowIdx++ {
		startIdx := windowIdx * stepSize
		endIdx := startIdx + config.FFTSize
		if endIdx > numComplexSamples {
			break
		}

		// Extract window and apply Hann window to complex samples
		window := make([]complex128, config.FFTSize)
		for i := 0; i < config.FFTSize; i++ {
			hannWeight := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(config.FFTSize-1)))
			window[i] = complexSamples[startIdx+i] * complex(hannWeight, 0)
		}

		// Perform complex FFT manually (Cooley-Tukey algorithm)
		coeffs := complexFFT(window)

		// FFT shift: rearrange so DC is in center and spectrum goes from -fs/2 to +fs/2
		shifted := make([]complex128, len(coeffs))
		half := len(coeffs) / 2
		for i := 0; i < len(coeffs); i++ {
			shifted[i] = coeffs[(i+half)%len(coeffs)]
		}

		// Calculate X position range - fill multiple columns per window for continuous display
		xStart := windowIdx * config.Width / numWindows
		xEnd := (windowIdx + 1) * config.Width / numWindows
		if xEnd > config.Width {
			xEnd = config.Width
		}
		if xStart >= config.Width {
			xStart = config.Width - 1
		}

		// Process each frequency bin
		for i := 0; i < numBins; i++ {
			binIdx := minBin + i
			if binIdx >= len(shifted) {
				break
			}

			// Calculate power spectrum density (magnitude squared)
			mag := cmplx.Abs(shifted[binIdx])
			powerDB := 10.0 * math.Log10(mag*mag+1e-12)

			// Normalize to 0-1 range using fixed dB scale
			normalized := (powerDB - minDB) / dbRange
			if normalized < 0 {
				normalized = 0
			}
			if normalized > 1 {
				normalized = 1
			}

			// Y axis: low frequencies at bottom, high at top
			y := config.Height - 1 - (i * config.Height / numBins)
			if y < 0 {
				y = 0
			}
			if y >= config.Height {
				y = config.Height - 1
			}

			// Apply smooth color map
			c := getWaterfallColor(normalized)

			// Fill the entire x range for this window (makes continuous bands instead of dots)
			for x := xStart; x < xEnd; x++ {
				img.Set(x, y, c)
			}
		}
	}

	return img, nil
}

// complexFFT performs FFT on complex input using Cooley-Tukey algorithm
func complexFFT(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	// Divide
	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = x[i*2]
		odd[i] = x[i*2+1]
	}

	// Conquer
	even = complexFFT(even)
	odd = complexFFT(odd)

	// Combine
	result := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Exp(complex(0, -2*math.Pi*float64(k)/float64(n))) * odd[k]
		result[k] = even[k] + t
		result[k+n/2] = even[k] - t
	}

	return result
}

// getWaterfallColor returns a color based on intensity (0.0 to 1.0)
// Implements a smooth, continuous colormap: dark blue -> blue -> cyan -> green -> yellow -> red -> white
func getWaterfallColor(intensity float64) color.Color {
	// Clamp intensity
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	var r, g, b float64

	if intensity < 0.2 {
		// Very dark blue to dark blue (0.0 - 0.2) - SDR noise floor
		t := intensity / 0.2
		r = 0
		g = 0
		b = 0.05 + 0.25*t // Start very dark: 0.05 -> 0.3
	} else if intensity < 0.4 {
		// Blue to cyan (0.2 - 0.4)
		t := (intensity - 0.2) / 0.2
		r = 0
		g = 0.5 * t     // 0 -> 0.5
		b = 0.5 + 0.5*t // 0.5 -> 1.0
	} else if intensity < 0.6 {
		// Cyan to green (0.4 - 0.6)
		t := (intensity - 0.4) / 0.2
		r = 0
		g = 0.5 + 0.5*t // 0.5 -> 1.0
		b = 1.0 - 0.5*t // 1.0 -> 0.5
	} else if intensity < 0.8 {
		// Green to yellow (0.6 - 0.8)
		t := (intensity - 0.6) / 0.2
		r = t           // 0 -> 1.0
		g = 1.0         // stays at 1.0
		b = 0.5 - 0.5*t // 0.5 -> 0
	} else if intensity < 0.9 {
		// Yellow to red (0.8 - 0.9)
		t := (intensity - 0.8) / 0.1
		r = 1.0         // stays at 1.0
		g = 1.0 - 0.5*t // 1.0 -> 0.5
		b = 0
	} else {
		// Red to white (0.9 - 1.0)
		t := (intensity - 0.9) / 0.1
		r = 1.0
		g = 0.5 + 0.5*t // 0.5 -> 1.0
		b = t           // 0 -> 1.0
	}

	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

// WriteWaterfallPNG writes a waterfall image as PNG to the given writer
func WriteWaterfallPNG(w io.Writer, samples []int16, config WaterfallConfig) error {
	img, err := GenerateWaterfall(samples, config)
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}
