package freepsgraph

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

const ROOT_SYMBOL = "_"

// Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	context   *base.Context
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*base.OperatorIO
	aborted   atomic.Bool
}

// NewGraph creates a new graph from a graph description
func NewGraph(ctx *base.Context, graphID string, origGraphDesc *GraphDesc, ge *GraphEngine) (*Graph, error) {
	if origGraphDesc == nil {
		return nil, errors.New("GraphDesc not set")
	}
	gd, err := origGraphDesc.GetCompleteDesc(graphID, ge)
	if err != nil {
		return nil, err
	}
	g := Graph{context: ctx, desc: gd, engine: ge, opOutputs: make(map[string]*base.OperatorIO), aborted: atomic.Bool{}}
	g.aborted.Store(false)
	return &g, nil
}

// GetCompleteDesc returns the GraphDesc that was sanitized and completed when creating the graph
func (g *Graph) GetCompleteDesc() *GraphDesc {
	return g.desc
}

// GetGraphID returns the unique ID for this graph in the graph engine
func (g *Graph) GetGraphID() string {
	return g.desc.GraphID
}

func (g *Graph) GetOperationTimeout() time.Duration {
	return time.Minute
}

func (g *Graph) GetTimeout() time.Duration {
	return 10 * time.Minute
}

func (g *Graph) executeSync(ctx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput
	logger := ctx.GetLogger()
	for i := 0; i < len(g.desc.Operations); i++ {
		if g.aborted.Load() {
			return base.MakeOutputError(http.StatusAlreadyReported, "Execution aborted")
		}
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

func (g *Graph) execute(ctx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	if g.GetTimeout() == 0 {
		return g.executeSync(ctx, mainArgs, mainInput)
	}

	c := make(chan *base.OperatorIO)
	var output *base.OperatorIO
	delay := time.NewTimer(g.GetTimeout())

	go func() {
		c <- g.executeSync(ctx, mainArgs, mainInput)
	}()

	select {
	case <-delay.C:
		g.aborted.Store(true)
		alertExpire := 5 * time.Minute
		output = base.MakeOutputError(http.StatusRequestTimeout, "Timeout when executing graph \"%v\" with arguments \"%v\"", g.desc.DisplayName, mainArgs)
		g.engine.SetSystemAlert(ctx, "graphTimeout", "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
		// clean up the timer
		if !delay.Stop() {
			// if the timer has been stopped then read from the channel.
			<-delay.C
		}
	}

	return output
}

func (g *Graph) collectAndReturnOperationError(ctx *base.Context, input *base.OperatorIO, opDesc *GraphOperationDesc, code int, msg string, a ...interface{}) *base.OperatorIO {
	error := base.MakeOutputError(code, msg, a...)
	g.engine.TriggerOnExecuteOperationHooks(ctx, input, error, g.GetGraphID(), opDesc)
	return error
}

func (g *Graph) executeOperation(ctx *base.Context, originalOpDesc *GraphOperationDesc, mainArgs base.FunctionArguments) *base.OperatorIO {
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
	if finalOpDesc.UseMainArgs {
		for k, v := range mainArgs.GetOriginalCaseMapOnlyFirst() {
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

		output := g.executeOperationWithOptionalTimeout(op, g.context, finalOpDesc.Function, base.NewFunctionArguments(finalOpDesc.Arguments), input)
		g.engine.TriggerOnExecuteOperationHooks(ctx, input, output, g.GetGraphID(), finalOpDesc)
		return output
	}
	return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "No operator with name \"%s\" found", finalOpDesc.Operator)
}

func (g *Graph) executeOperationWithOptionalTimeout(op base.FreepsBaseOperator, ctx *base.Context, fn string, mainArgs base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	if g.GetOperationTimeout() == 0 {
		return op.Execute(ctx, fn, mainArgs, input)
	}

	c := make(chan *base.OperatorIO)
	var output *base.OperatorIO
	delay := time.NewTimer(g.GetOperationTimeout())

	go func() {
		c <- op.Execute(ctx, fn, mainArgs, input)
	}()

	select {
	case <-delay.C:
		alertExpire := 5 * time.Minute
		output = base.MakeOutputError(http.StatusRequestTimeout, "Timeout when calling operator \"%v\", Function \"%v\" with arguments \"%v\"", op.GetName(), fn, mainArgs)
		g.engine.SetSystemAlert(ctx, "operationTimeout", "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
		// clean up the timer
		if !delay.Stop() {
			// if the timer has been stopped then read from the channel.
			<-delay.C
		}
	}

	return output
}
