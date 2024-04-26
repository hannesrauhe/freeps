package pixeldisplay

import (
	"image"
	"image/color"

	"github.com/hannesrauhe/freeps/base"
)

// Pixeldisplay is the interface for the pixel display, it provides functions to turn the display on and off, and to set the color, set a string of text, and set a RGB image.
type Pixeldisplay interface {
	// TurnOn turns the display on
	TurnOn() *base.OperatorIO
	// TurnOff turns the display off
	TurnOff() *base.OperatorIO
	// SetBrightness sets the brightness of the display
	SetBrightness(brightness int) *base.OperatorIO
	// SetEffect sets a pre-defined effect on the display
	SetEffect(fx int) *base.OperatorIO

	// SetPicture sets the picture of the display
	DrawImage(ctx *base.Context, image image.Image, returnPNG bool) *base.OperatorIO

	// SetBackgroundLayer sets a picture as background on the Display, if image is nil, the layer is deleted
	SetBackgroundLayer(ctx *base.Context, image image.Image, layerName string)
	// ResetBackground deletes all background layers
	ResetBackground(ctx *base.Context)
	// GetBackgroundLayerNames returns the names of active backgrounds
	GetBackgroundLayerNames() []string

	// SetColor sets the color of active pixels on the display
	SetColor(color color.Color)
	// SetBackground sets the color of inactive pixels on the display
	SetBackgroundColor(color color.Color)

	// GetMaxPictureSize returns the maximum size of a picture that can be displayed
	GetMaxPictureSize() image.Point
	// GetDimensions returns the dimensions of the display
	GetDimensions() image.Point
	// GetColor returns the color set for active pixels on the display
	GetColor() color.Color
	// GetBackground returns the color set for inactive pixels on the display
	GetBackgroundColor() color.Color
	// GetBrightness returns the brightness of the display
	GetBrightness() int

	// IsOn returns true if the display is on
	IsOn() bool

	Shutdown(ctx *base.Context)
}
