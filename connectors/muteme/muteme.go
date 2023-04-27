package muteme

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// MuteMe provides the interface to the muteme button and its LEDs
type MuteMe struct {
	impl *MuteMeImpl
}

// NewMuteMe creates a new MuteMe instance
func NewMuteMe(logger log.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*MuteMe, error) {
	mm := &MuteMe{}
	var err error
	mm.impl, err = newMuteMe(logger, cr, ge)
	return mm, err
}

// Shutdown the muteme listener
func (mm *MuteMe) Shutdown() {
	if mm.impl == nil {
		return
	}
	mm.impl.Shutdown()
	mm.impl = nil
}

func (mm *MuteMe) SetColor(color string) *base.OperatorIO {
	if mm.impl == nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Muteme not initialized")
	}
	if err := mm.impl.SetColor(color); err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Failed to set color: %v", err)
	}
	return base.MakePlainOutput(mm.impl.GetColor())
}

func (mm *MuteMe) GetColor() string {
	if mm.impl == nil {
		return "off"
	}
	return mm.impl.GetColor()
}

func (mm *MuteMe) StartListening() {
	if mm.impl != nil {
		go mm.impl.mainloop()
	}
}
