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

	// Define dB range for display (typical for spectrograms)
	const minDB = -80.0 // Noise floor
	const maxDB = -10.0 // Peak signals
	const dbRange = maxDB - minDB

	// Process each time window and draw directly
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

		// Calculate X position - distribute evenly across width
		x := windowIdx * config.Width / numWindows
		if x >= config.Width {
			x = config.Width - 1
		}

		// Process each frequency bin
		for i := 0; i < numBins; i++ {
			binIdx := minBin + i

			// Calculate power (magnitude squared)
			mag := cmplx.Abs(coeffs[binIdx])
			powerDB := 10.0 * math.Log10(mag*mag+1e-10)

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
			img.Set(x, y, c)
		}
	}

	return img, nil
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
		// Dark blue to blue (0.0 - 0.2)
		t := intensity / 0.2
		r = 0
		g = 0
		b = 0.1 + 0.4*t // 0.1 -> 0.5
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
