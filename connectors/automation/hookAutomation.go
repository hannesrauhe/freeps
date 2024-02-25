package automation

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookAutomation struct {
	oa *OpAutomation
}

var _ freepsgraph.FreepsGraphChangedHook = &HookAutomation{}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookAutomation) OnGraphChanged(ctx *base.Context, addedGraphName []string, removedGraphName []string) error {
	if h.oa == nil {
		return fmt.Errorf("Automation operator uninitialized")
	}
	h.oa.buildRuleAndTriggerMap()
	return nil
}
