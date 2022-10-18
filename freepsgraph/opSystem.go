package freepsgraph

import (
	"context"
	"net/http"
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/disk"
	"github.com/mackerelio/go-osstat/loadavg"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/mackerelio/go-osstat/network"
	"github.com/mackerelio/go-osstat/uptime"
)

type OpSystem struct {
	ge     *GraphEngine
	cancel context.CancelFunc
}

var _ FreepsOperator = &OpSystem{}

func NewSytemOp(ge *GraphEngine, cancel context.CancelFunc) *OpSystem {
	return &OpSystem{ge: ge, cancel: cancel}
}

func (o *OpSystem) Execute(fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "stop":
		fallthrough
	case "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
		return MakeEmptyOutput()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
		return MakeEmptyOutput()
	case "getGraphDesc":
		gd, ok := o.ge.GetGraphDesc(args["name"])
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "Unknown graph %v", args["name"])
		}
		return MakeObjectOutput(gd)
	case "getCollectedErrors":
		var err error
		duration := time.Hour
		if d, ok := args["duration"]; ok {
			duration, err = time.ParseDuration(d)
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, "Invalid duration %v", d)
			}
		}

		return MakeObjectOutput(o.ge.executionErrors.GetErrorsSince(duration))

	case "stats":
		var s interface{}
		var err error
		switch args["statType"] {
		case "cpu":
			s, err = cpu.Get()
		case "disk":
			s, err = disk.Get()
		case "loadavg":
			s, err = loadavg.Get()
		case "memory":
			s, err = memory.Get()
		case "network":
			s, err = network.Get()
		case "uptime ":
			s, err = uptime.Get()
		default:
			return MakeOutputError(http.StatusBadRequest, "unknown statType: "+args["statType"])
		}
		if err == nil {
			MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return MakeObjectOutput(s)
	}
	return MakeOutputError(http.StatusBadRequest, "Unknown function: "+fn)
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload", "stats", "getGraphDesc", "getCollectedErrors"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	switch fn {
	case "stats":
		return []string{"statType"}
	case "getGraphDesc":
		return []string{"name"}
	case "getCollectedErrors":
		return []string{"duration"}
	}
	return []string{}
}

func (o *OpSystem) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch fn {
	case "stats":
		switch arg {
		case "statType":
			return map[string]string{
				"cpu":     "cpu",
				"disk":    "disk",
				"loadavg": "loadavg",
				"memory":  "memory",
				"network": "network",
				"uptime":  "uptime",
			}
		}
	case "getGraphDesc":
		switch arg {
		case "name":
			agd := o.ge.GetAllGraphDesc()
			graphs := make(map[string]string)
			for n := range agd {
				graphs[n] = n
			}
			return graphs
		}
	case "getCollectedErrors":
		switch arg {
		case "duration":
			return map[string]string{
				"5m":  "5m",
				"10m": "10m",
				"1h":  "1h",
			}
		}
	}
	return map[string]string{}
}
