package freepsgraph

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

const ROOT_SYMBOL = "_"
const GraphTimeout = time.Minute * 2
const GraphOperationTimeout = time.Minute

// Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	context   *base.Context
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*base.OperatorIO
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
	g := Graph{context: ctx, desc: gd, engine: ge, opOutputs: make(map[string]*base.OperatorIO)}
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
	ds := g.desc.GetTagValue("operationTimeout")
	d, err := time.ParseDuration(ds)
	if ds == "" || err != nil {
		return GraphOperationTimeout
	}
	return d
}

func (g *Graph) GetTimeout() time.Duration {
	ds := g.desc.GetTagValue("graphTimeout")
	d, err := time.ParseDuration(ds)
	if ds == "" || err != nil {
		return GraphTimeout
	}
	return d
}

func (g *Graph) executeSync(ctx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput
	logger := ctx.GetLogger()
	for i := 0; i < len(g.desc.Operations); i++ {
		select {
		case <-ctx.Done():
			return base.MakeOutputError(http.StatusServiceUnavailable, "Execution aborted")
		default:
			operation := g.desc.Operations[i]
			output := g.executeOperation(ctx, &operation, mainArgs)
			logger.Debugf("Operation \"%s\" finished with output \"%v\"", operation.Name, output.ToString())
			g.opOutputs[operation.Name] = output
		}
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

func (g *Graph) execute(pctx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	if g.GetTimeout() == 0 {
		return g.executeSync(pctx, mainArgs, mainInput)
	}

	ctx, cancelFunc := base.WithTimeout(pctx, g.GetTimeout())
	defer cancelFunc()
	c := make(chan *base.OperatorIO)
	var output *base.OperatorIO

	startTime := time.Now()
	go func() {
		c <- g.executeSync(ctx, mainArgs, mainInput)
	}()

	select {
	case <-ctx.Done():
		alertExpire := 5 * time.Minute
		output = base.MakeOutputError(http.StatusGatewayTimeout, "Timeout after %v when executing graph \"%v\" with arguments \"%v\"", time.Now().Sub(startTime), g.desc.DisplayName, mainArgs)
		g.engine.SetSystemAlert(pctx, fmt.Sprintf("graphTimeout.%s", g.desc.GraphID), "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
	}

	return output
}

func (g *Graph) collectAndReturnOperationError(ctx *base.Context, input *base.OperatorIO, opDesc *GraphOperationDesc, code int, msg string, a ...interface{}) *base.OperatorIO {
	error := base.MakeOutputError(code, msg, a...)
	g.engine.TriggerOnExecuteOperationHooks(ctx, input, error, g.GetGraphID(), opDesc)
	return error
}

// replaceVariablesInArgs replaces variables of the form ${varName} in plainArgs with the values from the opOutputs
func (g *Graph) replaceVariablesInArgs(plainArgs map[string]string) (map[string]string, error) {
	r := make(map[string]string)

	if plainArgs == nil {
		return r, nil
	}

	var returnErr error

	re := regexp.MustCompile(`\${([^}]+)}`)
	for k, v := range plainArgs {
		r[k] = re.ReplaceAllStringFunc(v, func(match string) string {
			outputName := match[2 : len(match)-1]
			if opOutput, exists := g.opOutputs[outputName]; exists {
				return opOutput.GetString()
			}
			// split varName by "." and try to find the value in the opOutputs
			parts := strings.SplitN(outputName, ".", 2)
			if len(parts) != 2 {
				return ""
			}
			outputName = parts[0]
			varInMap := parts[1]
			opOutput, exists := g.opOutputs[outputName]
			if !exists {
				return ""
			}
			args, err := opOutput.GetArgsMap()
			if err != nil {
				returnErr = fmt.Errorf("Cannot get args from output \"%s\": %s", outputName, err)
				return ""
			}
			if val, exists := args[varInMap]; exists {
				return val
			}
			return ""
		})
	}
	return r, returnErr
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

	finalOpDesc := &GraphOperationDesc{}
	*finalOpDesc = *originalOpDesc
	var err error
	finalOpDesc.Arguments, err = g.replaceVariablesInArgs(originalOpDesc.Arguments)
	if err != nil {
		return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "Cannot create arguments: %s", err.Error())
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

		output := g.executeOperationWithOptionalTimeout(g.context, op, finalOpDesc.Function, base.NewFunctionArguments(finalOpDesc.Arguments), input)
		g.engine.TriggerOnExecuteOperationHooks(ctx, input, output, g.GetGraphID(), finalOpDesc)
		return output
	}
	return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "No operator with name \"%s\" found", finalOpDesc.Operator)
}

func (g *Graph) executeOperationWithOptionalTimeout(pctx *base.Context, op base.FreepsBaseOperator, fn string, mainArgs base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	if g.GetOperationTimeout() == 0 {
		return op.Execute(pctx, fn, mainArgs, input)
	}

	c := make(chan *base.OperatorIO)
	var output *base.OperatorIO
	ctx, cancelFunc := base.WithTimeout(pctx, g.GetOperationTimeout())
	defer cancelFunc()

	startTime := time.Now()
	go func() {
		c <- op.Execute(ctx, fn, mainArgs, input)
	}()

	select {
	case <-ctx.Done():
		alertExpire := 5 * time.Minute
		output = base.MakeOutputError(http.StatusGatewayTimeout, "Timeout after %v when calling operator \"%v\", Function \"%v\" with arguments \"%v\"", time.Now().Sub(startTime), op.GetName(), fn, mainArgs)
		g.engine.SetSystemAlert(pctx, fmt.Sprintf("operationTimeout.%s.%s", op.GetName(), fn), "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
	}

	return output
}
