package freepsflow

import (
	"context"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type OpSystem struct {
	ge     *FlowEngine
	cancel context.CancelFunc
}

var _ base.FreepsBaseOperator = &OpSystem{}

func NewSytemOp(ge *FlowEngine, cancel context.CancelFunc) *OpSystem {
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
	case "getFlow", "getFlowDesc", "GetFlowDesc":
		return o.ge.ExecuteOperatorByName(ctx, "flowbuilder", "getFlow", base.NewSingleFunctionArgument("flowName", args["name"]), base.MakeEmptyOutput())
	case "deleteFlow":
		return o.ge.ExecuteOperatorByName(ctx, "flowbuilder", "deleteFlow", base.NewSingleFunctionArgument("flowName", args["name"]), base.MakeEmptyOutput())
	case "GetFlowDescByTag":
		tags := []string{}
		if _, ok := args["tags"]; ok {
			tags = strings.Split(args["tags"], ",")
		}
		if args["tag"] != "" {
			tags = append(tags, args["tag"])
		}
		gim := o.ge.GetFlowDescByTag(tags)
		if gim == nil || len(gim) == 0 {
			return base.MakeOutputError(http.StatusNotFound, "No flows with tags %v", strings.Join(tags, ","))
		}
		return base.MakeObjectOutput(gim)

	case "stats":
		return o.Stats(ctx, fn, args, input)

	case "metrics":
		return base.MakeObjectOutput(o.ge.GetMetrics())

	case "noop":
		return base.MakeEmptyOutput()

	case "fail":
		return base.MakeOutputError(http.StatusExpectationFailed, "Fail requested")

	case "echo":
		if m, ok := args["output"]; ok {
			return base.MakePlainOutput(m)
		}
		return input

	case "hasInput":
		if input.IsEmpty() {
			return base.MakeOutputError(http.StatusBadRequest, "Expected input")
		}
		return input

	case "version":
		return base.MakePlainOutput(utils.BuildFullVersion())
	}
	return base.MakeOutputError(http.StatusBadRequest, "Unknown function: "+fn)
}

func (o *OpSystem) GetFunctions() []string {
	return []string{"shutdown", "reload", "stats", "getFlowDesc", "getFlowInfo", "getFlowDescByTag", "getCollectedErrors", "toDot", "contextToDot", "deleteFlow", "version", "metrics", "noop"}
}

func (o *OpSystem) GetPossibleArgs(fn string) []string {
	switch fn {
	case "stats":
		return []string{"statType"}
	case "getFlowDesc":
		return []string{"name"}
	case "GetFlowDesc":
		return []string{"name"}
	case "GetFlowDescByTag":
		return []string{"tags", "tag"}
	case "getCollectedErrors":
		return []string{"duration"}
	case "toDot":
		return []string{"name"}
	case "contextToDot":
		return []string{}
	case "deleteFlow":
		return []string{"name"}
	}
	return []string{"name"}
}

func (o *OpSystem) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	if arg == "name" {
		agd := o.ge.GetAllFlowDesc()
		flows := map[string]string{}
		for n := range agd {
			flows[n] = n
		}
		return flows
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
	case "getFlowDesc":
		fallthrough
	case "GetFlowDesc":
		switch arg {
		case "name":
			agd := o.ge.GetAllFlowDesc()
			flows := make(map[string]string)
			for n := range agd {
				flows[n] = n
			}
			return flows
		}
	case "GetFlowDescByTag":
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
