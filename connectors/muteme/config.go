//go:build !nomuteme && linux

package muteme

import "time"

type MuteMeConfig struct {
	Enabled            bool          // if false, the muteme button will be ignored
	MultiTouchDuration time.Duration // if touched multiple times within that duration, a separate graph will be called with the TouchCount
	LongTouchDuration  time.Duration // if touched once longer that than this, a separate graph will be called with the TouchDuration
	VendorID           uint16        // USB Vendor ID
	ProductID          uint16        // USB Product ID
	Tag                string        // tag that all graphs must have to be called
	TouchTag           string        // graphs with this tag will be called on a short single touch
	MultiTouchTag      string        // graphs with this tag will be called when button was touched multiple times within MultiTouchDuration
	LongTouchTag       string        // graphs with this tag will be called on a long single touch
	ProcessColor       string        // color to set while graphs are executed (if button is already in that color, turn light off instead)
	SuccessColor       string        // color to indicate successful graph execution
	ErrorColor         string        // colot to indicate error during graph execution
}

var DefaultConfig = MuteMeConfig{
	Enabled:            true,
	MultiTouchDuration: time.Second,
	LongTouchDuration:  3 * time.Second,
	VendorID:           0x20a0,
	ProductID:          0x42da,
	Tag:                "muteme",
	TouchTag:           "Touch",
	MultiTouchTag:      "MultiTouch",
	LongTouchTag:       "LongTouch",
	ProcessColor:       "purple",
	SuccessColor:       "green",
	ErrorColor:         "red",
}
