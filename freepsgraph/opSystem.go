package freepsgraph

import (
	"context"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
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
	case "GetGraphDesc":
		gi, ok := o.ge.GetGraphDesc(args["name"])
		if !ok {
			return base.MakeOutputError(http.StatusNotFound, "Unknown graph %v", args["name"])
		}
		return base.MakeObjectOutput(gi)
	case "toDot":
		g, out := o.ge.prepareGraphExecution(ctx, args["name"])
		if out.IsError() {
			return out
		}
		return base.MakeByteOutput(g.ToDot(ctx))
	case "GetGraphDescByTag":
		tags := []string{}
		if _, ok := args["tags"]; ok {
			tags = strings.Split(args["tags"], ",")
		}
		if args["tag"] != "" {
			tags = append(tags, args["tag"])
		}
		gim := o.ge.GetGraphDescByTag(tags)
		if gim == nil || len(gim) == 0 {
			return base.MakeOutputError(http.StatusNotFound, "No graphs with tags %v", strings.Join(tags, ","))
		}
		return base.MakeObjectOutput(gim)

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

	case "version":
		return base.MakePlainOutput(utils.BuildFullVersion())

	case "graphStats":
		return o.GraphStats(ctx, fn, args, input)
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown function: "+fn)
}

type GraphStats struct {
	OperatorCount  map[string]int
	FunctionsCount map[string]int
}

func (o *OpSystem) GraphStats(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	stats := GraphStats{OperatorCount: make(map[string]int), FunctionsCount: make(map[string]int)}
	g := o.ge.GetAllGraphDesc()
	for _, gd := range g {
		for _, op := range gd.Operations {
			stats.OperatorCount[op.Operator]++
			fn := op.Operator + "." + op.Function
			stats.FunctionsCount[fn]++
		}
	}
	return base.MakeObjectOutput(stats)
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload", "stats", "getGraphDesc", "getGraphInfo", "getGraphDescByTag", "getCollectedErrors", "toDot", "contextToDot", "deleteGraph", "version"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	switch fn {
	case "stats":
		return []string{"statType"}
	case "getGraphDesc":
		return []string{"name"}
	case "GetGraphDesc":
		return []string{"name"}
	case "GetGraphDescByTag":
		return []string{"tags", "tag"}
	case "getCollectedErrors":
		return []string{"duration"}
	case "toDot":
		return []string{"name"}
	case "contextToDot":
		return []string{}
	case "deleteGraph":
		return []string{"name"}
	case "graphStats":
		return []string{}
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
	case "GetGraphDesc":
		switch arg {
		case "name":
			agd := o.ge.GetAllGraphDesc()
			graphs := make(map[string]string)
			for n := range agd {
				graphs[n] = n
			}
			return graphs
		}
	case "GetGraphDescByTag":
		switch arg {
		case "tag":
			tags := o.ge.GetTags()
			return tags
		}
	}
	return map[string]string{}
}

// StartListening (noOp)
func (o *OpSystem) StartListening(ctx *base.Context) {
}

// Shutdown (noOp)
func (o *OpSystem) Shutdown(ctx *base.Context) {
}
