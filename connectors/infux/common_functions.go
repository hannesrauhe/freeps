//go:build !noinflux

package influx

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func (o *OperatorFlux) InitInflux(reinit bool) *base.OperatorIO {
	if o.writeApi != nil && !reinit {
		return base.MakeEmptyOutput()
	}

	connConfig := o.config
	influxOptions := influxdb2.DefaultOptions()
	client := influxdb2.NewClientWithOptions(connConfig.URL, connConfig.Token, influxOptions)
	if client == nil {
		return base.MakeOutputError(500, "Failed to create InfluxDB client, check connection settings")
	}
	o.writeApi = client.WriteAPI(connConfig.Org, connConfig.Bucket)
	if o.writeApi == nil {
		return base.MakeOutputError(500, "Failed to create InfluxDB write API, check connection settings")
	}
	return nil
}

func (o *OperatorFlux) PushFieldsInternal(measurement string, tags map[string]string, fields map[string]interface{}, ctx *base.Context) *base.OperatorIO {
	err := o.InitInflux(false)
	if err.IsError() {
		return err
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
