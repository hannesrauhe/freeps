//go:build nobluetooth

package freepsbluetooth

import (
	"fmt"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	"github.com/sirupsen/logrus"
)

var btwatcher *FreepsBluetooth

type FreepsBluetooth struct {
}

// StartDiscovery acts as a dummy
func (fbt *FreepsBluetooth) StartDiscovery() error {
	return nil
}

// StopDiscovery acts as a dummy
func (fbt *FreepsBluetooth) StopDiscovery(bool) {
}

// Shutdown acts as a dummy
func (fbt *FreepsBluetooth) Shutdown() {
}

// NewBTWatcher acts as a dummy
func NewBTWatcher(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*FreepsBluetooth, error) {
	return nil, fmt.Errorf("Bluetooth support not available")
}
