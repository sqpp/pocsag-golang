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
	FFTSize    int     // FFT window size (zero-padded) for high frequency resolution
	Overlap    float64 // Overlap between FFT windows (0.0 to 1.0)
	MinFreq    float64 // Minimum frequency to display (Hz)
	MaxFreq    float64 // Maximum frequency to display (Hz)
	SampleRate int     // Audio sample rate
	Colormap   string  // Colormap to use ("pysdr" or "legacy")
}

const (
	ColormapPySDR  = "pysdr"
	ColormapLegacy = "legacy"
)

// DefaultWaterfallConfig returns sensible defaults for POCSAG FSK waterfall
func DefaultWaterfallConfig() WaterfallConfig {
	return WaterfallConfig{
		Width:      1024, // MUST match FFTSize to avoid texture scaling issues in OpenGL
		Height:     600,  // Taller waterfall for better scrolling view
		FFTSize:    4096, // Large 4096 array padded with zeros for very high frequency resolution
		Overlap:    0.95,
		MinFreq:    -24000, // Show full spectrum (real SDR baseband)
		MaxFreq:    24000,
		SampleRate: SampleRate,
		Colormap:   ColormapPySDR,
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

	// For baseband I/Q, frequencies range from -fs/2 to +fs/2
	// After FFT shift, index 0 is -fs/2, and index FFTSize is +fs/2
	freqBinSize := float64(config.SampleRate) / float64(config.FFTSize) // e.g. 48000/1024 = 46.8Hz

	// Map actual frequencies to shifted FFT bin indices
	// -fs/2 -> bin 0. 0Hz -> bin N/2, +fs/2 -> bin N
	halfFs := float64(config.SampleRate) / 2.0
	minBin := int((config.MinFreq + halfFs) / freqBinSize)
	maxBin := int((config.MaxFreq + halfFs) / freqBinSize)

	if minBin < 0 {
		minBin = 0
	}
	if maxBin > config.FFTSize {
		maxBin = config.FFTSize
	}
	numBins := maxBin - minBin

	// Create output image
	img := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))

	// Define dB range for display
	// FSK signals are generated very strong to hit near 0 dB
	const minDB = -90.0 // Noise floor threshold for dark blue background
	const maxDB = 0.0   // Peak signals (red/white)
	const dbRange = maxDB - minDB

	// Process each time window (Y axis) and draw its frequency bins (X axis)
	for windowIdx := 0; windowIdx < numWindows; windowIdx++ {
		startIdx := windowIdx * stepSize
		endIdx := startIdx + config.FFTSize
		if endIdx > numComplexSamples {
			break
		}

		// Extract window and apply Hann window to complex samples
		window := make([]complex128, config.FFTSize)

		for i := 0; i < config.FFTSize; i++ {
			// Apply Hann window to smoothly taper the edges of this small chunk
			hannWeight := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(config.FFTSize-1)))
			window[i] = complexSamples[startIdx+i] * complex(hannWeight, 0)
		}

		// Perform complex FFT manually (Cooley-Tukey algorithm)
		coeffs := ComplexFFT(window)

		// Normalize FFT by window size (not FFT size) because the power only exists there
		for i := range coeffs {
			coeffs[i] /= complex(float64(config.FFTSize), 0)
		}

		// FFT shift: rearrange so DC is in center and spectrum goes from -fs/2 to +fs/2
		shifted := make([]complex128, len(coeffs))
		half := len(coeffs) / 2
		for i := 0; i < len(coeffs); i++ {
			shifted[i] = coeffs[(i+half)%len(coeffs)]
		}

		// Calculate Y position range (Time flows downward)
		// Y=0 is oldest (start of WAV), Y=config.Height is newest (end of WAV)
		yStart := windowIdx * config.Height / numWindows
		yEnd := (windowIdx + 1) * config.Height / numWindows
		if yEnd > config.Height {
			yEnd = config.Height
		}
		if yStart >= config.Height {
			yStart = config.Height - 1
		}

		// Process each frequency bin mapped to X axis
		for x := 0; x < config.Width; x++ {
			// Find corresponding frequency bin
			binIdx := minBin + (x * numBins / config.Width)
			if binIdx >= len(shifted) {
				binIdx = len(shifted) - 1
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

			// Apply smooth color map
			c := getWaterfallColor(normalized, config.Colormap)

			// Draw block (thick horizontal line for this time slice)
			for y := yStart; y < yEnd; y++ {
				img.Set(x, y, c)
			}
		}
	}

	return img, nil
}

// ComplexFFT performs FFT on complex input using Cooley-Tukey algorithm
func ComplexFFT(x []complex128) []complex128 {
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
	even = ComplexFFT(even)
	odd = ComplexFFT(odd)

	// Combine
	result := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Exp(complex(0, -2*math.Pi*float64(k)/float64(n))) * odd[k]
		result[k] = even[k] + t
		result[k+n/2] = even[k] - t
	}

	return result
}

// getWaterfallColor returns a color based on intensity (0.0 to 1.0) and chosen colormap
func getWaterfallColor(intensity float64, theme string) color.Color {
	if theme == ColormapLegacy {
		return getLegacyColor(intensity)
	}
	return getPySDRColor(intensity)
}

// getPySDRColor implements the colormap from the MLAB PySDR project.
// It maps intensity to a Blue -> Purple -> Red -> Yellow -> White scale.
func getPySDRColor(intensity float64) color.Color {
	// Clamp intensity
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	// Map 0..1 to the PySDR "a" range: -1.75 to 2.0
	// a = -1.75 is background (black/dark blue)
	// a = 2.0 is peak signal (white)
	a := intensity*3.75 - 1.75

	r := clamp(a+1.0, 0.0, 1.0)
	g := clamp(a, 0.0, 1.0)
	b := mag2colBase2Blue(a - 1.0)

	return color.RGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

// mag2colBase2Blue is the blue channel logic from PySDR's mag2col
func mag2colBase2Blue(val float64) float64 {
	if val <= -2.75 {
		return 0.0
	}
	if val <= -1.75 {
		return val + 2.75
	}
	if val <= -0.75 {
		return -(val + 0.75)
	}
	if val <= 0.0 {
		return 0.0
	}
	if val >= 1.0 {
		return 1.0
	}
	return val
}

// clamp helper
func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// getLegacyColor returns the original colormap: dark blue -> blue -> cyan -> green -> yellow -> red -> white
func getLegacyColor(intensity float64) color.Color {
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
