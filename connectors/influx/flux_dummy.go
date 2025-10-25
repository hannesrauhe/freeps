//go:build noinflux

package influx

import (
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
)

type OperatorInflux freepsflow.DummyOperator

var GlobalOperatorInflux *OperatorInflux = nil

func (o *OperatorInflux) PushFieldsInternal(measurement string, tags map[string]string, fields map[string]interface{}, ctx *base.Context) *base.OperatorIO {
	panic("InfluxDB db support not available, this should not be called")
}
