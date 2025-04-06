//go:build !noinflux

package influx

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func (o *OperatorInflux) PushFieldsInternal(measurement string, tags map[string]string, fields map[string]interface{}, ctx *base.Context) *base.OperatorIO {
	if o.writeApi == nil {
		return base.MakeOutputError(500, "InfluxDB write API not initialized")
	}

	if fields == nil || len(fields) == 0 {
		return base.MakeEmptyOutput()
	}

	p := influxdb2.NewPoint(measurement, tags, fields, time.Now())
	if p == nil {
		return base.MakeOutputError(500, "Failed to create InfluxDB point, check field types")
	}
	o.writeApi.WritePoint(p)
	return base.MakeEmptyOutput()
}
