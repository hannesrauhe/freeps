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

	// SetColor sets the color of active pixels on the display
	SetColor(color color.Color) *base.OperatorIO
	// SetBackground sets the color of inactive pixels on the display
	SetBackgroundColor(color color.Color) *base.OperatorIO
	// SetBrightness sets the brightness of the display
	SetBrightness(brightness int) *base.OperatorIO
	// SetEffect sets a pre-defined effect on the display
	SetEffect(fx int) *base.OperatorIO

	// SetPicture sets the picture of the display
	DrawImage(ctx *base.Context, image image.Image, returnPNG bool) *base.OperatorIO
	// SetBackground sets a picture as background on the Display
	SetBackgroundImage(ctx *base.Context, image image.Image) *base.OperatorIO
	// DrawPixel sets a pixel of the display
	DrawPixel(x, y int, color color.Color) *base.OperatorIO

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

	// GetImage returns the current image of the display
	GetImage() *image.RGBA
	// IsOn returns true if the display is on
	IsOn() bool

	Shutdown()
}
