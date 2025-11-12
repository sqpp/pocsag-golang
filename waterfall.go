package pocsag

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/dsp/fourier"
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

// DefaultWaterfallConfig returns sensible defaults for POCSAG
func DefaultWaterfallConfig() WaterfallConfig {
	return WaterfallConfig{
		Width:      2400, // Much wider for better time resolution
		Height:     256,
		FFTSize:    256,  // Smaller FFT for better time resolution
		Overlap:    0.75, // More overlap for smoother display
		MinFreq:    0,
		MaxFreq:    3000, // Only show 0-3kHz where POCSAG signal is
		SampleRate: SampleRate,
	}
}

// GenerateWaterfall creates a waterfall (spectrogram) image from audio samples
func GenerateWaterfall(samples []int16, config WaterfallConfig) (image.Image, error) {
	// Convert int16 samples to float64
	floatSamples := make([]float64, len(samples))
	for i, s := range samples {
		floatSamples[i] = float64(s) / 32768.0
	}

	// Calculate step size based on overlap
	stepSize := int(float64(config.FFTSize) * (1.0 - config.Overlap))
	numWindows := (len(floatSamples) - config.FFTSize) / stepSize

	if numWindows <= 0 {
		numWindows = 1
	}

	// Create FFT
	fft := fourier.NewFFT(config.FFTSize)

	// Calculate frequency bins
	freqBinSize := float64(config.SampleRate) / float64(config.FFTSize)
	minBin := int(config.MinFreq / freqBinSize)
	maxBin := int(config.MaxFreq / freqBinSize)
	if maxBin > config.FFTSize/2 {
		maxBin = config.FFTSize / 2
	}
	numBins := maxBin - minBin

	// Create output image
	img := image.NewRGBA(image.Rect(0, 0, config.Width, config.Height))

	// First pass: collect all magnitudes to find global min/max for normalization
	allMagnitudes := make([][]float64, 0, numWindows)
	globalMin := math.Inf(1)
	globalMax := math.Inf(-1)

	for windowIdx := 0; windowIdx < numWindows; windowIdx++ {
		startIdx := windowIdx * stepSize
		endIdx := startIdx + config.FFTSize
		if endIdx > len(floatSamples) {
			break
		}

		// Extract window and apply Hann window
		window := make([]float64, config.FFTSize)
		for i := 0; i < config.FFTSize; i++ {
			hannWeight := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(config.FFTSize-1)))
			window[i] = floatSamples[startIdx+i] * hannWeight
		}

		// Perform FFT
		coeffs := fft.Coefficients(nil, window)

		// Calculate magnitude spectrum
		magnitudes := make([]float64, numBins)
		for i := 0; i < numBins; i++ {
			binIdx := minBin + i
			mag := cmplx.Abs(coeffs[binIdx])
			// Convert to dB scale
			if mag > 1e-10 {
				magnitudes[i] = 20 * math.Log10(mag)
			} else {
				magnitudes[i] = -100 // Very low value for silence
			}
			if magnitudes[i] > globalMax {
				globalMax = magnitudes[i]
			}
			if magnitudes[i] < globalMin {
				globalMin = magnitudes[i]
			}
		}
		allMagnitudes = append(allMagnitudes, magnitudes)
	}

	// Ensure we have a valid range
	if globalMax <= globalMin {
		globalMax = globalMin + 1
	}

	// Second pass: draw the waterfall with proper normalization
	// Use a dynamic range of 60 dB for better contrast
	dynamicRange := 60.0
	threshold := globalMax - dynamicRange

	for windowIdx, magnitudes := range allMagnitudes {
		// Calculate X position - distribute evenly across width
		x := windowIdx * config.Width / len(allMagnitudes)
		if x >= config.Width {
			x = config.Width - 1
		}

		for freqIdx := 0; freqIdx < numBins; freqIdx++ {
			// Y axis: low frequencies at bottom, high at top
			y := config.Height - 1 - (freqIdx * config.Height / numBins)
			if y < 0 {
				y = 0
			}
			if y >= config.Height {
				y = config.Height - 1
			}

			// Normalize magnitude using dynamic range
			normalized := (magnitudes[freqIdx] - threshold) / dynamicRange
			if normalized < 0 {
				normalized = 0
			}
			if normalized > 1 {
				normalized = 1
			}

			// Apply color map
			c := getWaterfallColor(normalized)
			img.Set(x, y, c)
		}
	}

	return img, nil
}

// getWaterfallColor returns a color based on intensity (0.0 to 1.0)
// Uses a dark blue background with bright cyan/yellow/white for signals
func getWaterfallColor(intensity float64) color.Color {
	if intensity < 0.1 {
		// Very dark blue background (noise floor)
		return color.RGBA{0, 0, 15, 255}
	} else if intensity < 0.3 {
		// Dark blue
		t := (intensity - 0.1) / 0.2
		r := uint8(0)
		g := uint8(0)
		b := uint8(15 + t*50)
		return color.RGBA{r, g, b, 255}
	} else if intensity < 0.5 {
		// Blue to bright blue
		t := (intensity - 0.3) / 0.2
		r := uint8(0)
		g := uint8(t * 80)
		b := uint8(65 + t*135)
		return color.RGBA{r, g, b, 255}
	} else if intensity < 0.7 {
		// Bright blue to cyan
		t := (intensity - 0.5) / 0.2
		r := uint8(0)
		g := uint8(80 + t*175)
		b := uint8(200 + t*55)
		return color.RGBA{r, g, b, 255}
	} else if intensity < 0.85 {
		// Cyan to yellow/green
		t := (intensity - 0.7) / 0.15
		r := uint8(t * 255)
		g := uint8(255)
		b := uint8(255 - t*200)
		return color.RGBA{r, g, b, 255}
	} else {
		// Yellow to white (strongest signals)
		t := (intensity - 0.85) / 0.15
		r := uint8(255)
		g := uint8(255)
		b := uint8(55 + t*200)
		return color.RGBA{r, g, b, 255}
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
