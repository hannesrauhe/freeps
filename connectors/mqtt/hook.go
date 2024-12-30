package mqtt

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
)

type HookMQTT struct {
	impl *FreepsMqttImpl
}

var _ freepsflow.FreepsFlowChangedHook = &HookMQTT{}

// OnFlowChanged checks if subscriptions need to be changed
func (h *HookMQTT) OnFlowChanged(ctx *base.Context, addedFlowName []string, removedFlowName []string) error {
	return h.impl.startTagSubscriptions()
}
