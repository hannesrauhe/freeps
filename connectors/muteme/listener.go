package muteme

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsgraph"
)

// MuteMeListener provides methods to start listening for muteme events
type MuteMeListener struct {
}

// NewMuteMe creates a new MuteMe instance
func NewMuteMe() (*MuteMeListener, error) {
	if impl == nil {
		return nil, fmt.Errorf("Not Initialized")
	}
	mm := &MuteMeListener{}
	return mm, nil
}

// Shutdown the muteme listener
func (mm *MuteMeListener) Shutdown() {
	if impl == nil {
		return
	}
	impl.Shutdown()
	impl = nil
}

// StartListening starts the main loop of the muteme listener
func (mm *MuteMeListener) StartListening(ge *freepsgraph.GraphEngine) {
	if impl != nil {
		go impl.mainloop(ge)
	}
}
