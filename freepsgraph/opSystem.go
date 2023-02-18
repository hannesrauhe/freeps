package freepsgraph

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/utils"
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

// GetName returns the name of the operator
func (o *OpSystem) GetName() string {
	return "system"
}

func (o *OpSystem) Execute(ctx *utils.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	switch fn {
	case "stop", "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
		return MakeEmptyOutput()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
		return MakeEmptyOutput()
	case "getGraph", "getGraphDesc":
		gd, ok := o.ge.GetGraphDesc(args["name"])
		if !ok {
			return MakeOutputError(http.StatusNotFound, "Unknown graph %v", args["name"])
		}
		return MakeObjectOutput(gd)
	case "deleteGraph":
		err := o.ge.DeleteGraph(args["name"])
		if err != nil {
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return MakeEmptyOutput()
	case "getGraphInfo":
		gi, ok := o.ge.GetGraphInfo(args["name"])
		if !ok {
			return MakeOutputError(http.StatusNotFound, "Unknown graph %v", args["name"])
		}
		return MakeObjectOutput(gi)
	case "toDot":
		g, out := o.ge.prepareGraphExecution(ctx, args["name"], false)
		if out.IsError() {
			return out
		}
		return MakeByteOutput(g.ToDot(ctx))
	case "getGraphInfoByTag":
		tags := []string{}
		if _, ok := args["tags"]; ok {
			tags = strings.Split(args["tags"], ",")
		}
		if args["tag"] != "" {
			tags = append(tags, args["tag"])
		}
		gim := o.ge.GetGraphInfoByTag(tags)
		if gim == nil || len(gim) == 0 {
			return MakeOutputError(http.StatusNotFound, "No graphs with tags %v", strings.Join(tags, ","))
		}
		return MakeObjectOutput(gim)
	case "getCollectedErrors":
		var err error
		duration := time.Hour
		if d, ok := args["duration"]; ok {
			duration, err = time.ParseDuration(d)
			if err != nil {
				return MakeOutputError(http.StatusBadRequest, "Invalid duration %v", d)
			}
		}
		r := map[string]interface{}{"errors": o.ge.executionErrors.GetErrorsSince(duration)}
		return MakeObjectOutput(r)

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
	return []string{"shutdown", "reload", "stats", "getGraphDesc", "getGraphInfo", "getGraphInfoByTag", "getCollectedErrors", "deleteGraph"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	switch fn {
	case "stats":
		return []string{"statType"}
	case "getGraphDesc":
		return []string{"name"}
	case "getGraphInfo":
		return []string{"name"}
	case "getGraphInfoByTag":
		return []string{"tags", "tag"}
	case "getCollectedErrors":
		return []string{"duration"}
	}
	return []string{"name"}
}

func (o *OpSystem) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	if arg == "name" {
		agd := o.ge.GetAllGraphDesc()
		graphs := map[string]string{}
		for n := range agd {
			graphs[n] = n
		}
		return graphs
	}
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
		fallthrough
	case "getGraphInfo":
		switch arg {
		case "name":
			agd := o.ge.GetAllGraphDesc()
			graphs := make(map[string]string)
			for n := range agd {
				graphs[n] = n
			}
			return graphs
		}
	case "getGraphInfoByTag":
		switch arg {
		case "tag":
			tags := o.ge.GetTags()
			return tags
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

// Shutdown (noOp)
func (o *OpSystem) Shutdown(ctx *utils.Context) {
}
