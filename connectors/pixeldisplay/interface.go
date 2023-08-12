package pixeldisplay

import (
	"image"

	"github.com/hannesrauhe/freeps/base"
)

// Pixeldisplay is the interface for the pixel display, it provides functions to turn the display on and off, and to set the color, set a string of text, and set a RGB image.
type Pixeldisplay interface {
	// TurnOn turns the display on
	TurnOn() *base.OperatorIO
	// TurnOff turns the display off
	TurnOff() *base.OperatorIO

	// SetColor sets the color of active pixels on the display
	SetColor(color string) *base.OperatorIO
	// SetBackground sets the color of inactive pixels on the display
	SetBackground(color string) *base.OperatorIO
	// SetBrightness sets the brightness of the display
	SetBrightness(brightness int) *base.OperatorIO

	// SetText sets the text of the display
	SetText(text string) *base.OperatorIO
	// SetPicture sets the picture of the display
	SetImage(image *image.RGBA) *base.OperatorIO
	// SetPixel sets a pixel of the display
	SetPixel(x, y int, color string) *base.OperatorIO

	// GetDimensions returns the dimensions of the display
	GetDimensions() (width, height int)
	// GetColor returns the color set for active pixels on the display
	GetColor() string
	// GetBackground returns the color set for inactive pixels on the display
	GetBackground() string
	// GetText returns the current text of the display
	GetText() string
	// GetImage returns the current image of the display
	GetImage() *image.RGBA
	// IsOn returns true if the display is on
	IsOn() bool

	Shutdown()
}
