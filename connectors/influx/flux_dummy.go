//go:build noinflux

package influx

import "github.com/hannesrauhe/freeps/freepsflow"

type OperatorInflux freepsflow.DummyOperator

var GlobalOperatorInflux *OperatorInflux = nil
