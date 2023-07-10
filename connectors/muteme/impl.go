//go:build !nomuteme && linux

package muteme

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

type MuteMeImpl struct {
	dev          *hid.Device
	currentColor atomic.Value
	cmd          chan string
	config       *MuteMeConfig
	logger       logrus.FieldLogger
}

var impl *MuteMeImpl

func (m *MuteMeImpl) setColor(color string) error {
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

func (m *MuteMeImpl) blink(blinkColor string, afterColor string) error {
	for range []int{0, 1, 2, 3} {
		m.setColor("off")
		time.Sleep(100 * time.Millisecond)
		m.setColor(blinkColor)
		time.Sleep(100 * time.Millisecond)
	}
	m.setColor(afterColor)
}

func (m *MuteMeImpl) mainloop(ge *freepsgraph.GraphEngine) {
	bin := make([]byte, 8)
	tpress1 := time.Now()
	tpress2 := time.Now()
	ignoreUntil := time.Now()
	indicatorLightActive := false
	lastTouchDuration := time.Microsecond
	lastTouchCounter := 0
	running := true
	color := "off"

	// indicate startup by blinking:
	blink(m.config.SuccessColor, color)

	for running {
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
		}
		if bin[3] == 4 { // press
			lastTouchCounter++
			tpress1 = tpress2
			tpress2 = time.Now()
			if !indicatorLightActive {
				// make sure to not change the color multiple times
				indicatorLightActive = true
				color = m.GetColor()
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

		if bin[3] == 1 {
		} // pressed down
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

func (m *MuteMeImpl) Shutdown() {
	close(m.cmd)
}

func (m *MuteMeImpl) SetColor(color string) error {
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

func (m *MuteMeImpl) GetColor() string {
	return m.currentColor.Load().(string)
}

func newMuteMe(ctx *base.Context, mmc *MuteMeConfig) (*MuteMeImpl, error) {
	// Initialize the hid package.
	if err := hid.Init(); err != nil {
		return nil, err
	}

	// Open the device using the VID and PID.
	d, err := hid.OpenFirst(mmc.VendorID, mmc.ProductID)
	if err != nil {
		return nil, err
	}

	m := &MuteMeImpl{dev: d, cmd: make(chan string, 3), config: mmc, logger: ctx.GetLogger()}
	m.currentColor.Store("off")

	return m, nil
}
