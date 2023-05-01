package freepsgraph

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

type OpSystem struct {
	ge     *GraphEngine
	cancel context.CancelFunc
}

var _ base.FreepsBaseOperator = &OpSystem{}

func NewSytemOp(ge *GraphEngine, cancel context.CancelFunc) *OpSystem {
	return &OpSystem{ge: ge, cancel: cancel}
}

// GetName returns the name of the operator
func (o *OpSystem) GetName() string {
	return "system"
}

func (o *OpSystem) Execute(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	switch fn {
	case "stop", "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
		return base.MakeEmptyOutput()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
		return base.MakeEmptyOutput()
	case "getGraph", "getGraphDesc":
		gd, ok := o.ge.GetGraphDesc(args["name"])
		if !ok {
			return base.MakeOutputError(http.StatusNotFound, "Unknown graph %v", args["name"])
		}
		return base.MakeObjectOutput(gd)
	case "deleteGraph":
		err := o.ge.DeleteGraph(args["name"])
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		return base.MakeEmptyOutput()
	case "getGraphInfo":
		gi, ok := o.ge.GetGraphInfo(args["name"])
		if !ok {
			return base.MakeOutputError(http.StatusNotFound, "Unknown graph %v", args["name"])
		}
		return base.MakeObjectOutput(gi)
	case "toDot":
		g, out := o.ge.prepareGraphExecution(ctx, args["name"], false)
		if out.IsError() {
			return out
		}
		return base.MakeByteOutput(g.ToDot(ctx))
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
			return base.MakeOutputError(http.StatusNotFound, "No graphs with tags %v", strings.Join(tags, ","))
		}
		return base.MakeObjectOutput(gim)
	case "getCollectedErrors":
		var err error
		duration := time.Hour
		if d, ok := args["duration"]; ok {
			duration, err = time.ParseDuration(d)
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, "Invalid duration %v", d)
			}
		}
		r := map[string]interface{}{"errors": o.ge.executionErrors.GetErrorsSince(duration)}
		return base.MakeObjectOutput(r)

	case "contextToDot":
		var iCtx base.ContextNoTime
		if input.IsEmpty() {
			return base.MakeOutputError(http.StatusBadRequest, "No context to parse")
		}
		err := input.ParseJSON(&iCtx)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Unable to parse context: %v", err)
		}
		return base.MakePlainOutput(iCtx.ToDot())

	case "stats":
		return o.Stats(ctx, fn, args, input)
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown function: "+fn)
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload", "stats", "getGraphDesc", "getGraphInfo", "getGraphInfoByTag", "getCollectedErrors", "toDot", "contextToDot", "deleteGraph"}
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
func (o *OpSystem) Shutdown(ctx *base.Context) {
}
