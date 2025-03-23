//go:build linux

package freepsmetrics

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/disk"
	"github.com/mackerelio/go-osstat/loadavg"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/mackerelio/go-osstat/network"
	"github.com/mackerelio/go-osstat/uptime"
	"github.com/sirupsen/logrus"
)

func (s *StatsParams) StatTypeSuggestions() []string {
	return []string{"cpu", "disk", "loadavg", "memory", "network", "uptime"}
}

// Stats returns the system statistics on Linux
func (o *OpMetrics) Stats(ctx *base.Context, input *base.OperatorIO, args StatsParams) *base.OperatorIO {
	var s interface{}

	var err error
	statType := strings.ToLower(args.StatType)
	switch statType {
	case "cpu":
		s, err = cpu.Get()
	case "memory":
		s, err = memory.Get()
	case "loadavg":
		s, err = loadavg.Get()
	case "disk":
		ob := map[string]interface{}{}
		dstats, err := disk.Get()
		if err != nil {
			return base.MakeInternalServerErrorOutput(err)
		}
		for _, v := range dstats {
			ob[v.Name] = v
		}
		s = ob
	case "network":
		ob := map[string]interface{}{}
		nstats, err := network.Get()
		if err != nil {
			return base.MakeInternalServerErrorOutput(err)
		}
		for _, v := range nstats {
			ob[v.Name] = v
		}
		s = ob
	case "uptime":
		uptime, err := uptime.Get()
		if err != nil {
			return base.MakeInternalServerErrorOutput(err)
		}
		ob := map[string]interface{}{}
		ob["uptime"] = uptime
		ob["uptime_seconds"] = uptime.Seconds()
		ob["uptime_readable"] = fmt.Sprintf("%v", uptime)
		s = ob
	default:
		return base.MakeOutputError(http.StatusBadRequest, "unknown statType: %s", args.StatType)
	}
	if err != nil {
		base.MakeInternalServerErrorOutput(err)
	}

	ret := base.MakeObjectOutput(s)
	opSensor := sensor.GetGlobalSensors()
	if opSensor != nil {
		err = opSensor.SetSensorPropertyFromFlattenedObject(ctx, "system_stats", statType, s)
		if err != nil {
			logrus.Warnf("Cannot set sensor properties for the %v stats, error: %v", statType, err)
		}
	}
	return ret
}
