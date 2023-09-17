package freepsgraph

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hannesrauhe/freeps/base"
)

const ROOT_SYMBOL = "_"

// Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	name      string
	context   *base.Context
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*base.OperatorIO
}

// NewGraph creates a new graph from a graph description
func NewGraph(ctx *base.Context, name string, origGraphDesc *GraphDesc, ge *GraphEngine) (*Graph, error) {
	if ge == nil {
		return nil, errors.New("GraphEngine not set")
	}
	if origGraphDesc == nil {
		return nil, errors.New("GraphDesc not set")
	}
	if len(origGraphDesc.Operations) == 0 {
		return nil, errors.New("No operations defined")
	}
	gd := *origGraphDesc
	gd.Operations = make([]GraphOperationDesc, len(origGraphDesc.Operations))

	outputNames := make(map[string]bool)
	outputNames[ROOT_SYMBOL] = true
	// create a copy of each operation and add it to the graph
	for i, op := range origGraphDesc.Operations {
		if op.Name == ROOT_SYMBOL {
			return nil, errors.New("Operation name cannot be " + ROOT_SYMBOL)
		}
		if outputNames[op.Name] {
			return nil, errors.New("Operation name " + op.Name + " is used multiple times")
		}
		if op.Name == "" {
			op.Name = fmt.Sprintf("#%d", i)
		}
		if !ge.HasOperator(op.Operator) {
			return nil, fmt.Errorf("Operation \"%v\" references unknown operator \"%v\"", op.Name, op.Operator)
		}
		if op.ArgumentsFrom != "" && outputNames[op.ArgumentsFrom] != true {
			return nil, fmt.Errorf("Operation \"%v\" references unknown argumentsFrom \"%v\"", op.Name, op.ArgumentsFrom)
		}
		if op.InputFrom == "" && i == 0 {
			op.InputFrom = ROOT_SYMBOL
		}
		if op.InputFrom != "" && outputNames[op.InputFrom] != true {
			return nil, fmt.Errorf("Operation \"%v\" references unknown inputFrom \"%v\"", op.Name, op.InputFrom)
		}
		if op.ExecuteOnFailOf != "" {
			if outputNames[op.ExecuteOnFailOf] != true {
				return nil, fmt.Errorf("Operation \"%v\" references unknown ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
			if op.ExecuteOnFailOf == op.InputFrom {
				return nil, fmt.Errorf("Operation \"%v\" references the same InputFrom and ExecuteOnFailOf \"%v\"", op.Name, op.ExecuteOnFailOf)
			}
		}
		outputNames[op.Name] = true
		gd.Operations[i] = op

		// op.args are not copied, because they aren't modified during execution
	}
	if origGraphDesc.OutputFrom == "" {
		if len(origGraphDesc.Operations) == 1 {
			gd.OutputFrom = gd.Operations[0].Name
		}
	} else if outputNames[origGraphDesc.OutputFrom] != true {
		return nil, fmt.Errorf("Graph references unknown outputFrom \"%v\"", origGraphDesc.OutputFrom)
	}
	return &Graph{name: name, context: ctx, desc: &gd, engine: ge, opOutputs: make(map[string]*base.OperatorIO)}, nil
}

// GetCompleteDesc returns the GraphDesc that was sanitized and completed when creating the graph
func (g *Graph) GetCompleteDesc() *GraphDesc {
	return g.desc
}

func (g *Graph) execute(ctx *base.Context, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	ctx.IncreaseNesting()
	defer ctx.DecreaseNesting()
	g.opOutputs[ROOT_SYMBOL] = mainInput
	logger := ctx.GetLogger()
	for i := 0; i < len(g.desc.Operations); i++ {
		operation := g.desc.Operations[i]
		output := g.executeOperation(ctx, &operation, mainArgs)
		logger.Debugf("Operation \"%s\" finished with output \"%v\"", operation.Name, output.ToString())
		g.opOutputs[operation.Name] = output
	}
	if g.desc.OutputFrom == "" {
		return base.MakeObjectOutput(g.opOutputs)
	}
	if g.opOutputs[g.desc.OutputFrom] == nil {
		logger.Errorf("Output from \"%s\" not found", g.desc.OutputFrom)
		return base.MakeObjectOutput(g.opOutputs)
	}
	return g.opOutputs[g.desc.OutputFrom]
}

func (g *Graph) collectAndReturnOperationError(ctx *base.Context, input *base.OperatorIO, opDesc *GraphOperationDesc, code int, msg string, a ...interface{}) *base.OperatorIO {
	error := base.MakeOutputError(code, msg, a...)
	g.engine.TriggerOnExecutionErrorHooks(ctx, input, error, g.name, opDesc)
	return error
}

func (g *Graph) executeOperation(ctx *base.Context, originalOpDesc *GraphOperationDesc, mainArgs map[string]string) *base.OperatorIO {
	logger := ctx.GetLogger()
	input := base.MakeEmptyOutput()
	if originalOpDesc.InputFrom != "" {
		input = g.opOutputs[originalOpDesc.InputFrom]
		if input.IsError() {
			// reduce logging of eval-related "errors"
			if input.HTTPCode != http.StatusExpectationFailed {
				logger.Debugf("Not executing executing operation \"%v\", because \"%v\" returned an error", originalOpDesc.Name, originalOpDesc.InputFrom)
			}
			return input
		}
	}

	if originalOpDesc.ExecuteOnFailOf != "" && !g.opOutputs[originalOpDesc.ExecuteOnFailOf].IsError() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Operation not executed because \"%v\" did not fail", originalOpDesc.ExecuteOnFailOf)
	}

	// create a copy of the arguments for collecting possible errors
	finalOpDesc := &GraphOperationDesc{}
	*finalOpDesc = *originalOpDesc
	finalOpDesc.Arguments = make(map[string]string)
	if originalOpDesc.Arguments != nil {
		for k, v := range originalOpDesc.Arguments {
			finalOpDesc.Arguments[k] = v
		}
	}
	if finalOpDesc.IgnoreMainArgs == false {
		for k, v := range mainArgs {
			if _, ok := finalOpDesc.Arguments[k]; ok {
				logger.Warnf("Argument %s overwritten by main arg", k)
			}
			finalOpDesc.Arguments[k] = v
		}
	}

	if finalOpDesc.ArgumentsFrom != "" {
		outputToBeArgs, exists := g.opOutputs[finalOpDesc.ArgumentsFrom]
		if !exists {
			return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "Output of \"%s\" cannot be used as arguments, because there is no such output", finalOpDesc.ArgumentsFrom)
		}
		if outputToBeArgs.IsError() {
			// reduce logging of eval-related "errors"
			if outputToBeArgs.HTTPCode != http.StatusExpectationFailed {
				logger.Debugf("Not executing executing operation \"%v\", because \"%v\" returned an error", finalOpDesc.Name, finalOpDesc.InputFrom)
			}
			return input
		}
		collectedArgs, err := outputToBeArgs.GetArgsMap()
		if err != nil {
			return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 500, "Output of \"%s\" cannot be used as arguments: %v", finalOpDesc.ArgumentsFrom, err)
		}
		for k, v := range collectedArgs {
			finalOpDesc.Arguments[k] = v
		}
	}

	op := g.engine.GetOperator(finalOpDesc.Operator)
	if op != nil {
		logger.Debugf("Calling operator \"%v\", Function \"%v\" with arguments \"%v\"", finalOpDesc.Operator, finalOpDesc.Function, finalOpDesc.Arguments)
		opI := ctx.RecordOperationStart(g.name, finalOpDesc.Operator+"."+finalOpDesc.Function, finalOpDesc.Name, finalOpDesc.InputFrom)
		g.engine.TriggerOnExecuteOperationHooks(ctx, opI)
		output := op.Execute(g.context, finalOpDesc.Function, finalOpDesc.Arguments, input)
		if output.IsError() {
			g.engine.TriggerOnExecutionErrorHooks(ctx, input, output, g.name, finalOpDesc)
		}
		ctx.RecordOperationFinish(opI, output.HTTPCode)
		return output
	}
	return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "No operator with name \"%s\" found", finalOpDesc.Operator)
}

// ToQuicklink returns the URL to call a standalone-operation outside of a Graph
func (gop *GraphOperationDesc) ToQuicklink() string {
	var s strings.Builder
	s.WriteString("/" + gop.Operator)
	if gop.Function != "" {
		s.WriteString("/" + gop.Function)
	}
	if len(gop.Arguments) > 0 {
		s.WriteString("?")
	}
	for k, v := range gop.Arguments {
		s.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&")
	}
	return s.String()
}
