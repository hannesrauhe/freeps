//go:build !nomuteme && linux

package muteme

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/sstallion/go-hid"
)

// MuteMe implements the FreepsOperator interface to control the MuteMe button
type MuteMe struct {
	GE           *freepsflow.FlowEngine
	config       MuteMeConfig
	dev          *hid.Device
	currentColor atomic.Value
	cmd          chan string
	ctx          *base.Context
}

var _ base.FreepsOperatorWithConfig = &MuteMe{}
var _ base.FreepsOperatorWithShutdown = &MuteMe{}

// GetDefaultConfig returns a copy of the default config
func (mm *MuteMe) GetDefaultConfig() interface{} {
	newConfig := DefaultConfig
	return &newConfig
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (mm *MuteMe) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	mmc := *config.(*MuteMeConfig)

	// Initialize the hid package.
	if err := hid.Init(); err != nil {
		return nil, err
	}

	// Open the device using the VID and PID.
	d, err := hid.OpenFirst(mmc.VendorID, mmc.ProductID)
	if err != nil {
		return nil, err
	}
	newMM := MuteMe{config: mmc, GE: mm.GE, dev: d, cmd: make(chan string, 3), ctx: ctx}
	newMM.currentColor.Store("off")

	return &newMM, nil
}

func (m *MuteMe) setColorImpl(color string) error {
	if len(m.cmd) >= cap(m.cmd) {
		return fmt.Errorf("Channel is over capacity")
	}
	if _, ok := colors[color]; !ok {
		return fmt.Errorf("%v is not a valid color", color)
	}

	select {
	case m.cmd <- color:
		return nil
	default:
		return fmt.Errorf("Channel was closed")
	}
}

func (m *MuteMe) getColorImpl() string {
	return m.currentColor.Load().(string)
}

// SetColorArgs are the arguments for the MuteMe-SetColor function
type SetColorArgs struct {
	Color string
}

// ColorSuggestions returns suggestions for the color
func (mma *SetColorArgs) ColorSuggestions() []string {
	r := make([]string, 0, len(colors))
	for c := range colors {
		r = append(r, c)
	}
	return r
}

// SetColor sets the color of the MuteMe button
func (mm *MuteMe) SetColor(ctx *base.Context, input *base.OperatorIO, args SetColorArgs) *base.OperatorIO {
	if err := mm.setColorImpl(args.Color); err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
	}
	gs := sensor.GetGlobalSensors()
	if gs != nil {
		gs.SetSensorPropertyInternal(ctx, mm.config.Tag, mm.config.Tag, "Color", args.Color)
	}
	return base.MakePlainOutput(mm.getColorImpl())
}

// TurnOff turns off the MuteMe button
func (mm *MuteMe) TurnOff(ctx *base.Context) *base.OperatorIO {
	return mm.SetColor(ctx, nil, SetColorArgs{Color: "off"})
}

// Cycle cycles through the colors of the MuteMe button
func (mm *MuteMe) Cycle(ctx *base.Context) *base.OperatorIO {
	for c, b := range colors {
		if b != 0x00 && c != mm.getColorImpl() {
			return mm.SetColor(ctx, nil, SetColorArgs{Color: c})
		}
	}
	return base.MakePlainOutput(mm.getColorImpl())
}

// GetColor returns the current color of the MuteMe button
func (mm *MuteMe) GetColor() *base.OperatorIO {
	return base.MakePlainOutput(mm.getColorImpl())
}

// Shutdown the muteme listener
func (mm *MuteMe) Shutdown(ctx *base.Context) {
	// indicate shutdown by blinking:
	mm.blink(mm.config.ErrorColor, "off")

	close(mm.cmd)
}

func (mm *MuteMe) outerLoop() {
	running := true
	for running {
		mm.mainloop(&running)
		if running {
			//there was an error, wait a second before reinit
			time.Sleep(time.Second)
		}
	}
	mm.ctx.GetLogger().Info("MuteMe background thread stopped")
}

// StartListening starts the main loop of the muteme listener
func (mm *MuteMe) StartListening(ctx *base.Context) {
	go mm.outerLoop()
}
