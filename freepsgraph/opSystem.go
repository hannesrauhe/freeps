package freepsgraph

import (
	"context"
	"net/http"

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
	case "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
		return MakeEmptyOutput()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
		return MakeEmptyOutput()
	case "getGraph":
		gd, ok := o.ge.GetGraphDesc(args["name"])
		if !ok {
			return MakeOutputError(http.StatusBadRequest, "Unknown graph %v", args["name"])
		}
		return MakeObjectOutput(gd)
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
	return []string{"shutdown", "reload", "stats", "getGraphDesc"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	return []string{"statType"}
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
	}
	return map[string]string{}
}
