//go:build !nomuteme && linux

package muteme

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

// MuteMe implements the FreepsOperator interface to control the MuteMe button
type MuteMe struct {
	GE           *freepsgraph.GraphEngine
	config       MuteMeConfig
	dev          *hid.Device
	currentColor atomic.Value
	cmd          chan string
	logger       logrus.FieldLogger
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
	d, err := hid.OpenFirst(mm.config.VendorID, mm.config.ProductID)
	if err != nil {
		return nil, err
	}
	newMM := MuteMe{config: mmc, GE: mm.GE, dev: d, cmd: make(chan string, 3), logger: ctx.GetLogger()}
	newMM.currentColor.Store("off")

	return &newMM, nil
}

func (m *MuteMe) setColor(color string) error {
	b := make([]byte, 2)
	b[0] = 0x0

	ok := false
	b[1], ok = colors[color]
	if !ok {
		b[1] = 0x07
		color = "white"
	}

	_, err := m.dev.Write(b)
	lColor := m.currentColor.Load().(string)
	if err == nil && color != lColor {
		m.currentColor.Store(color)
	}
	if err != nil {
		m.logger.Errorf("Error setting color: %v", err)
	}
	return err
}

func (m *MuteMe) blink(blinkColor string, afterColor string) {
	for range []int{0, 1, 2, 3} {
		m.setColor("off")
		time.Sleep(100 * time.Millisecond)
		m.setColor(blinkColor)
		time.Sleep(100 * time.Millisecond)
	}
	m.setColor(afterColor)
}

func (m *MuteMe) mainloop(ge *freepsgraph.GraphEngine) {
	bin := make([]byte, 8)
	tpress1 := time.Now()
	tpress2 := time.Now()
	ignoreUntil := time.Now()
	indicatorLightActive := false
	longTouchLightActive := false
	lastTouchDuration := time.Microsecond
	lastTouchCounter := 0
	running := true
	color := "off"

	// indicate startup by blinking:
	m.blink(m.config.SuccessColor, color)

	for running {
		// set the user-requested color unless the indicator light is active
		if !indicatorLightActive {
			select {
			case str, open := <-m.cmd:
				if !open {
					running = false
				} else {
					m.setColor(str)
				}
				continue
			default:
				// nothing to do
			}
		}
		_, err := m.dev.ReadWithTimeout(bin, m.config.MultiTouchDuration)
		if time.Now().Before(ignoreUntil) {
			if bin[3] == 4 {
				// make sure we don't execute something twice because the user was too slow when double touching
				ignoreUntil = time.Now().Add(time.Second)
			}
			continue
		}
		if err != nil {
			// should be a timeout error in normal operation
			if !errors.Is(err, hid.ErrTimeout) {
				// it's another error, usually interrupted system call. Nothing to do but ignore
				// logrus.Errorf("Error getting state: %v", err)
				continue
			}

			if lastTouchDuration <= time.Microsecond {
				// nothing happened
				continue
			}

			// action:
			tags := []string{m.config.Tag}
			args := map[string]string{}
			if tpress2.Sub(tpress1) < m.config.MultiTouchDuration {
				tags = append(tags, m.config.MultiTouchTag)
				args["TouchCount"] = fmt.Sprint(lastTouchCounter)
			} else {
				if lastTouchDuration > m.config.LongTouchDuration {
					tags = append(tags, m.config.LongTouchTag)
				} else {
					tags = append(tags, m.config.TouchTag)
				}
				args["TouchDuration"] = lastTouchDuration.String()
			}
			resultIO := ge.ExecuteGraphByTags(base.NewContext(m.logger), tags, args, base.MakeEmptyOutput())
			ignoreUntil = time.Now().Add(time.Second)
			m.logger.Debugf("Muteme touched, result: %v", resultIO)
			if resultIO.IsError() {
				m.blink(m.config.ErrorColor, color)
			} else {
				m.blink(m.config.SuccessColor, color)
			}

			// reset state variables
			lastTouchDuration = time.Microsecond
			lastTouchCounter = 0
			indicatorLightActive = false
			longTouchLightActive = false
		}
		if bin[3] == 4 { // press
			lastTouchCounter++
			tpress1 = tpress2
			tpress2 = time.Now()
			if !indicatorLightActive {
				// make sure to not change the color multiple times
				indicatorLightActive = true
				color = m.getColorImpl()
				if color != m.config.ProcessColor {
					m.setColor(m.config.ProcessColor)
				} else {
					m.setColor("off")
				}
			}
		}
		if bin[3] == 2 { // release
			lastTouchDuration = time.Now().Sub(tpress2)
		}

		if bin[3] == 1 { // pressed down
			if time.Now().Sub(tpress2) > m.config.LongTouchDuration && !longTouchLightActive {
				longTouchLightActive = true
				m.setColor(m.config.LongTouchColor)
			}
		}
		if bin[3] == 0 {
		} // no touch
	}

	if err := m.dev.Close(); err != nil {
		m.logger.Error(err)
	}

	if err := hid.Exit(); err != nil {
		m.logger.Error(err)
	}
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

// GetArgSuggestions returns suggestions for the color
func (mma *SetColorArgs) GetArgSuggestions(op base.FreepsOperator, fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// SetColor sets the color of the MuteMe button
func (mm *MuteMe) SetColor(ctx *base.Context, input *base.OperatorIO, args SetColorArgs) *base.OperatorIO {
	if err := mm.setColorImpl(args.Color); err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
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

// StartListening starts the main loop of the muteme listener
func (mm *MuteMe) StartListening(ctx *base.Context) {
	go mm.mainloop(mm.GE)
}
