//go:build !cgo
// +build !cgo

package pocsag

import (
	"errors"
)

// WaterfallGL is a stub for non-CGO builds
type WaterfallGL struct {
	width  int
	height int
}

// NewWaterfallGL returns an error on non-CGO builds
func NewWaterfallGL(width, height int, headless bool) (*WaterfallGL, error) {
	return nil, errors.New("OpenGL waterfall support requires CGO to be enabled")
}

// AddLine is a stub
func (w *WaterfallGL) AddLine(data []float32) {}

// Render is a stub
func (w *WaterfallGL) Render() {}

// Close is a stub
func (w *WaterfallGL) Close() {}

// SaveToPNG returns an error on non-CGO builds
func (w *WaterfallGL) SaveToPNG(filename string) error {
	return errors.New("OpenGL waterfall support requires CGO to be enabled")
}

func (w *WaterfallGL) ShouldClose() bool {
	return true
}
