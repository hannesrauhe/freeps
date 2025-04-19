//go:build !noinflux

package influx

import (
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

func (o *OperatorInflux) PushFieldsInternal(measurement string, tags map[string]string, fields map[string]interface{}, ctx *base.Context) *base.OperatorIO {
	if fields == nil || len(fields) == 0 {
		return base.MakeEmptyOutput()
	}

	p := influxdb2.NewPoint(measurement, tags, fields, time.Now())
	if p == nil {
		return base.MakeOutputError(500, "Failed to create InfluxDB point, check field types")
	}

	if o.storeNamespace != nil {
		b := strings.Builder{}
		write.PointToLineProtocolBuffer(p, &b, time.Second)
		o.storeNamespace.SetValue("", base.MakePlainOutput(b.String()), ctx)

		return base.MakeEmptyOutput()
	}

	if o.writeApi == nil {
		return base.MakeOutputError(500, "InfluxDB write API not initialized")
	}
	o.writeApi.WritePoint(p)
	return base.MakeEmptyOutput()
}
