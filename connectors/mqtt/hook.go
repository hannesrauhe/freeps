package mqtt

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookMQTT struct {
	impl *FreepsMqttImpl
}

var _ freepsgraph.FreepsGraphChangedHook = &HookMQTT{}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookMQTT) OnGraphChanged(ctx *base.Context, addedGraphName []string, removedGraphName []string) error {
	return h.impl.startTagSubscriptions()
}
