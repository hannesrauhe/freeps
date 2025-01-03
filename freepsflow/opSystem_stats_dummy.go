//go:build windows

package freepsflow

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

// Stats on Windows returns an error
func (o *OpSystem) Stats(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "Stats only available on Linux")
}
