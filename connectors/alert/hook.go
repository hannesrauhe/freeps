package opalert

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookAlert struct {
	impl *OpAlert
}

var _ freepsgraph.FreepsAlertHook = &HookAlert{}

// OnSystemAlert registers alerts send to the GraphEngine (allows other operators to set alerts)
func (h *HookAlert) OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) error {
	errStr := err.Error()
	a := Alert{Name: name, Category: &category, Severity: &severity, Desc: &errStr, ExpiresAt: nil}
	if expiresIn != nil {
		eTime := time.Now().Add(*expiresIn)
		a.ExpiresAt = &eTime
	}
	h.impl.SetAlert(ctx, base.MakeEmptyOutput(), a)
	return nil
}

// OnResetSystemAlert unsets alerts send to the GraphEngine (allows other operators to set/reset alerts)
func (h *HookAlert) OnResetSystemAlert(ctx *base.Context, name string, category string) error {
	a := Alert{Name: name, Category: &category}
	h.impl.ResetAlert(ctx, base.MakeEmptyOutput(), a)
	return nil
}
