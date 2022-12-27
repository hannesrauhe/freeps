package usb

import (
	"fmt"
	"log"
	"time"

	logrus "github.com/sirupsen/logrus"
	"github.com/sstallion/go-hid"
)

type MuteMe struct {
	dev          *hid.Device
	currentColor string
	lastColor    string
	done         chan bool
}

var colors = map[string]byte{
	"red":     0x01,
	"green":   0x02,
	"blue":    0x04,
	"yellow":  0x03,
	"cyan":    0x06,
	"purple":  0x05,
	"white":   0x07,
	"nocolor": 0x00,
	"off":     0x00,
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
	if err == nil {
		m.lastColor = m.currentColor
		m.currentColor = color
	}
	return err
}

func (m *MuteMe) mainloop() {
	bin := make([]byte, 8)
	tpress1 := time.Now()
	tpress2 := time.Now()
	lastPressed := time.Microsecond
	running := true
	for running {
		select {
		case <-m.done:
			running = false
		default:
		}
		_, err := m.dev.Read(bin)
		if err != nil {
			if lastPressed > time.Microsecond {
				fmt.Printf("Pressed: %v\n", lastPressed)
				lastPressed = time.Microsecond
			}
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if bin[3] == 4 { // press
			tpress1 = tpress2
			tpress2 = time.Now()
			m.setColor("red")
		}
		if bin[3] == 2 { // release
			lastPressed = time.Now().Sub(tpress2)
			time.Sleep(time.Millisecond * 200)
			m.setColor(m.lastColor)
		}

		if bin[3] == 0 { // no touch
			if tpress2.Sub(tpress1) < time.Millisecond*1000 {
				fmt.Println("Doublepress")
				lastPressed = time.Microsecond
			} else {
				// fmt.Printf("Pressed: %v\n", lastPressed)
			}
		}
	}

	if err := m.dev.Close(); err != nil {
		logrus.Error(err)
	}

	if err := hid.Exit(); err != nil {
		logrus.Error(err)
	}
}

func (m *MuteMe) Shutdown() {
	m.done <- true
}

func NewMuteMe() *MuteMe {
	// Initialize the hid package.
	if err := hid.Init(); err != nil {
		log.Fatal(err)
	}

	// Open the device using the VID and PID.
	d, err := hid.OpenFirst(0x20a0, 0x42da)
	if err != nil {
		log.Fatal(err)
	}

	// // Read the Manufacturer String.
	// s, err := d.GetMfrStr()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Printf("Manufacturer String: %s\n", s)

	// // Read the Product String.
	// s, err = d.GetProductStr()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Printf("Product String: %s\n", s)

	d.SetNonblock(true)

	// Toggle LED (cmd 0x80). The first byte is the report number (0x0).
	// b[0] = 0x0
	// b[1] = 0x04
	// if _, err := d.Write(b); err != nil {
	// 	log.Fatal(err)
	// }

	// oldbin3 := byte(0)
	m := &MuteMe{dev: d, currentColor: "off"}
	m.setColor("blue")
	go m.mainloop()

	return m
}
