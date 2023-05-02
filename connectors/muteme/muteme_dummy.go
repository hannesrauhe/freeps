//go:build nomuteme || windows

package muteme

import (
	"fmt"
	"github.com/hannesrauhe/freeps/base"
)

type MuteMeConfig struct {
	Enabled bool // if false, the muteme button will be ignored
}

var DefaultConfig = MuteMeConfig{
	Enabled: false,
}

type MuteMeImpl struct{}

var impl *MuteMeImpl

func (m *MuteMeImpl) SetColor(color string) error {
	return fmt.Errorf("Not compiled")
}

func (m *MuteMeImpl) GetColor() string {
	return "off"
}

func (m *MuteMeImpl) Shutdown() {
}

func (m *MuteMeImpl) mainloop(interface{}) {
}

func newMuteMe(ctx *base.Context, mmc *MuteMeConfig) (*MuteMeImpl, error) {
	return nil, fmt.Errorf("Not compiled")
}
