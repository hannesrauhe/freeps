//go:build !muteme
// +build !muteme

package muteme

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	logrus "github.com/sirupsen/logrus"
)

type MuteMeImpl struct{}

func (m *MuteMeImpl) SetColor(color string) error {
	return fmt.Errorf("Not compiled")
}

func (m *MuteMeImpl) GetColor() string {
	return "off"
}

func newMuteMe(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*MuteMeImpl, error) {
	return nil, fmt.Errorf("Not compiled")
}
