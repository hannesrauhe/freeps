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

func (o *OpSystem) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return o.ExecuteOld(ctx, fn, fa.GetOriginalCaseMapJoined(), input)
}

func (o *OpSystem) ExecuteOld(ctx *base.Context, fn string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	switch fn {
	case "stop", "shutdown":
		o.ge.reloadRequested = false
		o.cancel()
		return base.MakeEmptyOutput()
	case "reload":
		o.ge.reloadRequested = true
		o.cancel()
		return base.MakeEmptyOutput()
	case "getGraph", "getGraphDesc", "GetGraphDesc":
		return o.ge.ExecuteOperatorByName(ctx, "graphbuilder", "getGraph", base.NewSingleFunctionArgument("graphName", args["name"]), base.MakeEmptyOutput())
	case "deleteGraph":
		return o.ge.ExecuteOperatorByName(ctx, "graphbuilder", "deleteGraph", base.NewSingleFunctionArgument("graphName", args["name"]), base.MakeEmptyOutput())
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

	case "stats":
		return o.Stats(ctx, fn, args, input)

	case "metrics":
		return base.MakeObjectOutput(o.ge.GetMetrics())

	case "noop":
		return base.MakeEmptyOutput()

	case "version":
		return base.MakePlainOutput(utils.BuildFullVersion())
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown function: "+fn)
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload", "stats", "getGraphDesc", "getGraphInfo", "getGraphDescByTag", "getCollectedErrors", "toDot", "contextToDot", "deleteGraph", "version", "metrics", "noop"}
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

// GetHook (noOp)
func (o *OpSystem) GetHook() interface{} {
	return nil
}
