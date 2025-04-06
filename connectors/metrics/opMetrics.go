package freepsmetrics

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

type OpMetrics struct {
	CR     *utils.ConfigReader
	GE     *freepsflow.FlowEngine
	ticker *time.Ticker
}

var _ base.FreepsOperatorWithShutdown = &OpMetrics{}

var updateInterval = 5 * time.Minute // not really worth configuring for now

func (o *OpMetrics) FreepsMetrics(ctx *base.Context) *base.OperatorIO {
	metrics := map[string]interface{}{
		"Version":        utils.Version,
		"CommitHash":     utils.CommitHash,
		"BuildTime":      utils.BuildTime,
		"Branch":         utils.Branch,
		"StartTimestamp": utils.StartTimestamp,
		"Runtime":        fmt.Sprintf("%v", time.Since(utils.StartTimestamp)),
	}
	ob, err := utils.ObjectToMap(o.GE.GetMetrics())
	if err == nil {
		for k, v := range ob {
			metrics[k] = v
		}
	}
	opSensor := sensor.GetGlobalSensors()
	if opSensor != nil {
		err = opSensor.SetSensorPropertiesInternal(ctx, "freeps", "metrics", metrics)
		if err != nil {
			logrus.Warnf("Cannot set sensor properties for freepsmetrics, error: %v", err)
		}
	}
	return base.MakeObjectOutput(metrics)
}

type StatsParams struct {
	StatType string
}

func (o *OpMetrics) CPUStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "cpu"})
}

func (o *OpMetrics) MemoryStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "memory"})
}

func (o *OpMetrics) LoadAvgStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "loadavg"})
}

func (o *OpMetrics) DiskStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "disk"})
}

func (o *OpMetrics) NetworkStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "network"})
}

func (o *OpMetrics) UptimeStats(ctx *base.Context) *base.OperatorIO {
	return o.Stats(ctx, base.MakeEmptyOutput(), StatsParams{StatType: "uptime"})
}

func (o *OpMetrics) triggerAllMetrics(ctx *base.Context) {
	o.FreepsMetrics(ctx)
	o.CPUStats(ctx)
	o.MemoryStats(ctx)
	o.LoadAvgStats(ctx)
	o.DiskStats(ctx)
	o.NetworkStats(ctx)
	o.UptimeStats(ctx)
}

func (o *OpMetrics) loop(initCtx *base.Context) {
	o.triggerAllMetrics(initCtx)

	if o.ticker == nil {
		return
	}

	for range o.ticker.C {
		ctx := base.CreateContextWithField(initCtx, "component", "freepsmetrics", "periodic loop")

		o.triggerAllMetrics(ctx)
	}
}

func (o *OpMetrics) StartListening(ctx *base.Context) {
	if o.ticker != nil {
		return
	}
	o.ticker = time.NewTicker(updateInterval)
	go o.loop(ctx)
}

func (o *OpMetrics) Shutdown(ctx *base.Context) {
	if o.ticker == nil {
		return
	}
	o.ticker.Stop()
	o.ticker = nil
}
