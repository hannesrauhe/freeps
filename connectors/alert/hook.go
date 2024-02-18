package opalert

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookAlert struct {
	impl *OpAlert
}

var _ freepsgraph.FreepsAlertHook = &HookAlert{}

// OnGraphChanged checks if subscriptions need to be changed
func (h *HookAlert) OnSystemAlert(ctx *base.Context, category string, name string, severity int, err error) error {
	errStr := err.Error()
	a := Alert{Name: name, Category: &category, Severity: &severity, Desc: &errStr}
	h.impl.SetAlert(ctx, base.MakeEmptyOutput(), a)
	return nil
}
