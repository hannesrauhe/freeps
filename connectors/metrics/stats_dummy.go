//go:build windows

package freepsmetrics

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

// Stats on Windows returns an error
func (o *OpMetrics) Stats(ctx *base.Context, input *base.OperatorIO, args StatsParams) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "Stats only available on Linux")
}
