// +build muteme

package muteme

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

type MuteMeImpl struct {
	dev          *hid.Device
	ge           *freepsgraph.GraphEngine
	currentColor atomic.Value
	lastColor    string
	cmd          chan string
	config       *MuteMeConfig
	logger       logrus.FieldLogger
}

type MuteMeConfig struct {
	DoublePressTime  time.Duration
	VendorID         uint16
	ProductID        uint16
	PressGraph       string
	DoublePressGraph string
}

var DefaultConfig = MuteMeConfig{
	DoublePressTime:  time.Second,
	VendorID:         0x20a0,
	ProductID:        0x42da,
	PressGraph:       "muteMePressed",
	DoublePressGraph: "muteMeDoublePress",
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
		m.lastColor = lColor
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
	lastPressed := time.Microsecond
	doublePressTime := m.config.DoublePressTime
	running := true
	for running {
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
		_, err := m.dev.ReadWithTimeout(bin, doublePressTime)
		if time.Now().Before(ignoreUntil) {
			if bin[3] == 4 {
				// fmt.Println("Ignored")
				ignoreUntil = time.Now().Add(time.Second)
			}
			continue
		}
		if err != nil {
			if !errors.Is(err, hid.ErrTimeout) {
				// usually interrupted system call. Nothing to do but ignore
				// logrus.Errorf("Error getting state: %v", err)
				continue
			}
			if lastPressed <= time.Microsecond {
				continue
			}

			if tpress2.Sub(tpress1) < doublePressTime {
				// fmt.Println("Doublepress")
				m.ge.ExecuteGraph(utils.NewContext(m.logger), m.config.DoublePressGraph, map[string]string{}, freepsgraph.MakeEmptyOutput())
			} else {
				m.ge.ExecuteGraph(utils.NewContext(m.logger), m.config.PressGraph, map[string]string{"time": lastPressed.String()}, freepsgraph.MakeEmptyOutput())
				// fmt.Printf("Pressed: %v\n", lastPressed)
			}
			ignoreUntil = time.Now().Add(time.Second)
			lastPressed = time.Microsecond
			m.setColor(m.lastColor)
		}
		if bin[3] == 4 { // press
			tpress1 = tpress2
			tpress2 = time.Now()
			if m.GetColor() != "red" {
				m.setColor("red")
			} else {
				m.setColor("off")
			}
		}
		if bin[3] == 2 { // release
			lastPressed = time.Now().Sub(tpress2)
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
