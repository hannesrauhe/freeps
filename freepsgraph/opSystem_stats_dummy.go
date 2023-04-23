//go:build windows

package freepsgraph

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

// Stats on Windows returns an error
func (o *OpSystem) Stats(ctx *base.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	return MakeOutputError(http.StatusNotImplemented, "Stats only available on Linux")
}
