//go:build !nomuteme && linux

package muteme

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

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
	if err != nil {
		m.logger.Errorf("Error setting color: %v", err)
		return err
	}
	m.currentColor.Store(color)
	return nil
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

func (m *MuteMe) mainloop(running *bool) {
	bin := make([]byte, 8)
	tpress1 := time.Now()
	tpress2 := time.Now()
	ignoreUntil := time.Now()
	indicatorLightActive := false
	longTouchLightActive := false
	lastTouchDuration := time.Microsecond
	lastTouchCounter := 0
	color := "off"
	alertCategory := "system"
	alertName := "mutemeOffline"
	ctx := base.NewContext(m.logger)
	if m.dev == nil {
		// Open the device using the VID and PID.
		d, err := hid.OpenFirst(m.config.VendorID, m.config.ProductID)
		if err != nil {
			alertError := fmt.Errorf("MuteMe is offline because: %w", err)
			m.GE.SetSystemAlert(ctx, alertName, alertCategory, 2, alertError, nil)
			return
		}
		m.dev = d
	}

	// indicate startup by blinking:
	m.blink(m.config.SuccessColor, color)

	for *running {
		// set the user-requested color unless the indicator light is active
		if !indicatorLightActive {
			select {
			case str, open := <-m.cmd:
				if !open {
					*running = false
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
			// should be a timeout error in normal operation, or an interupt
			if !errors.Is(err, hid.ErrTimeout) && !strings.Contains(err.Error(), "Interrupted system call") {
				alertError := fmt.Errorf("MuteMe is offline because: %w", err)
				m.GE.SetSystemAlert(ctx, alertName, alertCategory, 2, alertError, nil)
				logrus.Errorf("Error getting state: %v", err)
				break
			}
			m.GE.ResetSystemAlert(ctx, alertName, alertCategory)

			if lastTouchDuration <= time.Microsecond {
				// nothing happened
				continue
			}

			// action:
			resultIO := m.execTriggers(ctx, tpress2.Sub(tpress1), lastTouchDuration, lastTouchCounter)
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

	m.dev = nil
	if err := hid.Exit(); err != nil {
		m.logger.Error(err)
	}
}
