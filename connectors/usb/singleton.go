package usb

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	"fmt"
	"sync"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// MuteMe provides the functions for MQTT handling
type MuteMe struct {
	impl *MuteMeImpl
}

var instantiated *MuteMe
var once sync.Once

// GetInstance returns the process-wide instance of MuteMe, instance needs to be initialized before use
func GetInstance() *MuteMe {
	once.Do(func() {
		instantiated = &MuteMe{}
	})
	return instantiated
}

// Init initilaizes FreepsMQTT based on the config
func (mm *MuteMe) Init(logger log.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) error {
	if mm.impl != nil {
		return fmt.Errorf("MuteMe already initialized")
	}
	var err error
	mm.impl, err = newMuteMe(logger, cr, ge)
	return err
}

// Shutdown MQTT and cancel all subscriptions
func (mm *MuteMe) Shutdown() {
	if mm.impl == nil {
		return
	}
	mm.impl = nil
}

func (mm *MuteMe) SetColor(color string) *freepsgraph.OperatorIO {
	if mm.impl == nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Muteme not initialized")
	}
	if err := mm.impl.SetColor(color); err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
	}
	return freepsgraph.MakeEmptyOutput()
}

func (mm *MuteMe) GetColor() string {
	if mm.impl == nil {
		return "off"
	}
	return mm.impl.GetColor()
}
