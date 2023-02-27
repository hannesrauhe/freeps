//go:build !nomuteme
// +build !nomuteme

package muteme

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

type MuteMeImpl struct {
	dev          *hid.Device
	ge           *freepsgraph.GraphEngine
	currentColor atomic.Value
	cmd          chan string
	config       *MuteMeConfig
	logger       logrus.FieldLogger
}

type MuteMeConfig struct {
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

func (m *MuteMeImpl) mainloop() {
	bin := make([]byte, 8)
	tpress1 := time.Now()
	tpress2 := time.Now()
	ignoreUntil := time.Now()
	indicatorLightActive := false
	lastTouchDuration := time.Microsecond
	lastTouchCounter := 0
	running := true
	color := "off"
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
			resultIO := m.ge.ExecuteGraphByTags(base.NewContext(m.logger), tags, args, freepsgraph.MakeEmptyOutput())
			ignoreUntil = time.Now().Add(time.Second)
			m.logger.Debugf("Muteme touched, result: %v", resultIO)
			resultIndicatorColor := m.config.SuccessColor
			if resultIO.IsError() {
				resultIndicatorColor = m.config.ErrorColor
			}
			for range []int{0, 1, 2, 3} {
				m.setColor("off")
				time.Sleep(100 * time.Millisecond)
				m.setColor(resultIndicatorColor)
				time.Sleep(100 * time.Millisecond)
			}

			// reset state variables
			lastTouchDuration = time.Microsecond
			lastTouchCounter = 0
			indicatorLightActive = false
			m.setColor(color)
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

func newMuteMe(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*MuteMeImpl, error) {
	mmc := DefaultConfig
	err := cr.ReadSectionWithDefaults("muteme", &mmc)
	if err != nil {
		return nil, err
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		logrus.Print(err)
	}

	// Initialize the hid package.
	if err := hid.Init(); err != nil {
		return nil, err
	}

	// Open the device using the VID and PID.
	d, err := hid.OpenFirst(mmc.VendorID, mmc.ProductID)
	if err != nil {
		return nil, err
	}

	m := &MuteMeImpl{dev: d, cmd: make(chan string, 3), config: &mmc, logger: logrus.StandardLogger(), ge: ge}
	m.currentColor.Store("off")

	return m, nil
}
