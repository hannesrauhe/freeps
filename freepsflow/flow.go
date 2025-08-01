package freepsflow

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
const FlowTimeout = time.Minute * 2
const FlowOperationTimeout = time.Minute

// Flow is the instance created from a FlowDesc and contains the runtime data
type Flow struct {
	desc      *FlowDesc
	engine    *FlowEngine
	opOutputs map[string]*base.OperatorIO
}

// NewFlow creates a new flow from a flow description
func NewFlow(ctx *base.Context, flowID string, origFlowDesc *FlowDesc, ge *FlowEngine) (*Flow, error) {
	if origFlowDesc == nil {
		return nil, errors.New("FlowDesc not set")
	}
	gd, err := origFlowDesc.GetCompleteDesc(flowID, ge)
	if err != nil {
		return nil, err
	}
	g := Flow{desc: gd, engine: ge, opOutputs: make(map[string]*base.OperatorIO)}
	return &g, nil
}

// GetCompleteDesc returns the FlowDesc that was sanitized and completed when creating the flow
func (g *Flow) GetCompleteDesc() *FlowDesc {
	return g.desc
}

// GetFlowID returns the unique ID for this flow in the flow engine
func (g *Flow) GetFlowID() string {
	return g.desc.FlowID
}

func (g *Flow) GetOperationTimeout() time.Duration {
	ds := g.desc.GetTagValue("operationTimeout")
	d, err := time.ParseDuration(ds)
	if ds == "" || err != nil {
		return FlowOperationTimeout
	}
	return d
}

func (g *Flow) GetTimeout() time.Duration {
	ds := g.desc.GetTagValue("flowTimeout")
	d, err := time.ParseDuration(ds)
	if ds == "" || err != nil {
		return FlowTimeout
	}
	return d
}

func (g *Flow) executeSync(parentCtx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	g.engine.metrics.FlowExecutions++
	ctx := parentCtx.ChildContextWithField("flow", g.desc.FlowID)
	if g.desc.HasAllTags([]string{"debuglogging"}) {
		prevLevel := ctx.EnableDebugLogging()
		defer ctx.DisableDebugLogging(prevLevel)
	}
	ctx.GetLogger().Debugf("Executing flow \"%s\"(\"%s\") with arguments \"%v\"", g.desc.FlowID, g.desc.DisplayName, mainArgs)
	defer ctx.GetLogger().Debugf("Flow \"%s\" finished", g.desc.FlowID)

	g.opOutputs[ROOT_SYMBOL] = mainInput
	logger := ctx.GetLogger()
	for i := 0; i < len(g.desc.Operations); i++ {
		select {
		case <-ctx.Done():
			return base.MakeOutputError(http.StatusServiceUnavailable, "Execution aborted")
		default:
			operation := g.desc.Operations[i]
			output := g.executeOperation(ctx, &operation, mainArgs)
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
	flowOutput := g.opOutputs[g.desc.OutputFrom]
	ctx.GetLogger().Debugf("Flow \"%s\" finished with output \"%v\"", g.desc.DisplayName, flowOutput.ToString())
	return flowOutput
}

func (g *Flow) execute(pctx *base.Context, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) *base.OperatorIO {
	if g.GetTimeout() == 0 {
		return g.executeSync(pctx, mainArgs, mainInput)
	}

	ctx, cancelFunc := pctx.ChildContextWithTimeout(g.GetTimeout())
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
		output = base.MakeOutputError(http.StatusGatewayTimeout, "Timeout after %v when executing flow \"%v\" with arguments \"%v\"", time.Now().Sub(startTime), g.desc.DisplayName, mainArgs)
		g.engine.SetSystemAlert(pctx, fmt.Sprintf("flowTimeout.%s", g.desc.FlowID), "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
	}

	return output
}

func (g *Flow) collectAndReturnOperationError(ctx *base.Context, input *base.OperatorIO, opDesc *FlowOperationDesc, code int, msg string, a ...interface{}) *base.OperatorIO {
	ctx.GetLogger().Debugf("Operation \"%s\" failed with error \"%s\"", opDesc.Name, fmt.Sprintf(msg, a...))
	error := base.MakeOutputError(code, msg, a...)
	g.engine.TriggerOnExecuteOperationHooks(ctx, input, error, g.GetFlowID(), opDesc)
	return error
}

// replaceVariablesInArgs replaces variables of the form ${varName} in plainArgs with the values from the opOutputs
func (g *Flow) replaceVariablesInArgs(plainArgs base.FunctionArguments) (base.FunctionArguments, error) {
	r := make(map[string]string)

	if plainArgs == nil {
		return base.MakeEmptyFunctionArguments(), nil
	}

	var returnErr error

	re := regexp.MustCompile(`\${([^}]+)}`)
	for k, v := range plainArgs.GetLowerCaseMapJoined() {
		r[k] = re.ReplaceAllStringFunc(v, func(match string) string {
			outputName := match[2 : len(match)-1]
			if opOutput, exists := g.opOutputs[outputName]; exists {
				return opOutput.GetString()
			}
			// split varName by "." and try to find the value in the opOutputs
			parts := strings.SplitN(outputName, ".", 2)
			if len(parts) < 2 {
				returnErr = fmt.Errorf("Output \"%s\" not found", outputName)
				return ""
			}
			outputName = parts[0]
			varInMap := parts[1]
			opOutput, exists := g.opOutputs[outputName]
			if !exists {
				returnErr = fmt.Errorf("Output \"%s\" not found", outputName)
				return ""
			}
			args, err := opOutput.GetArgsMap()
			if err != nil {
				returnErr = fmt.Errorf("Cannot get \"%s\" from \"%s\": %s", varInMap, outputName, err)
				return ""
			}
			val, exists := args[varInMap]
			if !exists {
				returnErr = fmt.Errorf("Variable \"%s\" not found in output \"%s\"", varInMap, outputName)
				return ""
			}
			return val
		})
	}
	return base.NewFunctionArguments(r), returnErr
}

func (g *Flow) executeOperation(parentCtx *base.Context, originalOpDesc *FlowOperationDesc, mainArgs base.FunctionArguments) *base.OperatorIO {
	g.engine.metrics.OperationExecutions++
	ctx := parentCtx.ChildContextWithField("operation", originalOpDesc.Name)
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

	if originalOpDesc.ExecuteOnSuccessOf != "" && g.opOutputs[originalOpDesc.ExecuteOnSuccessOf].IsError() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Operation not executed because \"%v\" did not succeed", originalOpDesc.ExecuteOnSuccessOf)
	}

	if originalOpDesc.ExecuteOnFailOf != "" && !g.opOutputs[originalOpDesc.ExecuteOnFailOf].IsError() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Operation not executed because \"%v\" did not fail", originalOpDesc.ExecuteOnFailOf)
	}

	finalOpDesc := &FlowOperationDesc{}
	*finalOpDesc = *originalOpDesc
	var err error
	finalOpDesc.FunctionArgs, err = g.replaceVariablesInArgs(originalOpDesc.FunctionArgs)
	if err != nil {
		return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "%s", err.Error())
	}

	combinedArgs := finalOpDesc.FunctionArgs
	if finalOpDesc.UseMainArgs {
		for k, v := range mainArgs.GetOriginalCaseMap() {
			if combinedArgs.Has(k) {
				logger.Warnf("Argument %s overwritten by main arg", k)
			}
			combinedArgs.Set(k, v)
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
			combinedArgs.Set(k, []string{v})
		}
	}

	op := g.engine.GetOperator(finalOpDesc.Operator)
	if op != nil {
		ctx.GetLogger().Debugf("Calling operator \"%v\", Function \"%v\" with arguments \"%v\"", finalOpDesc.Operator, finalOpDesc.Function, combinedArgs.GetOriginalCaseMap())
		defer ctx.GetLogger().Debugf("Operation \"%s\" finished", originalOpDesc.Name)

		output := g.executeOperationWithOptionalTimeout(ctx, op, finalOpDesc.Function, combinedArgs, input)
		g.engine.TriggerOnExecuteOperationHooks(ctx, input, output, g.GetFlowID(), finalOpDesc)
		return output
	}
	return g.collectAndReturnOperationError(ctx, input, finalOpDesc, 404, "No operator with name \"%s\" found", finalOpDesc.Operator)
}

func (g *Flow) executeOperationWithOptionalTimeout(parentCtx *base.Context, op base.FreepsBaseOperator, fn string, mainArgs base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	fnctx := parentCtx.ChildContextWithField("op-fn", op.GetName()+"/"+fn)
	if g.GetOperationTimeout() == 0 {
		return op.Execute(fnctx, fn, mainArgs, input)
	}

	c := make(chan *base.OperatorIO)
	var output *base.OperatorIO
	ctxWithTimeout, cancelFunc := fnctx.ChildContextWithTimeout(g.GetOperationTimeout())
	defer cancelFunc()

	startTime := time.Now()
	go func() {
		c <- op.Execute(ctxWithTimeout, fn, mainArgs, input)
	}()

	select {
	case <-ctxWithTimeout.Done():
		alertExpire := 5 * time.Minute
		output = base.MakeOutputError(http.StatusGatewayTimeout, "Timeout after %v when calling operator \"%v\", Function \"%v\" with arguments \"%v\"", time.Now().Sub(startTime), op.GetName(), fn, mainArgs)
		g.engine.SetSystemAlert(parentCtx, fmt.Sprintf("operationTimeout.%s.%s", op.GetName(), fn), "system", 2, output.GetError(), &alertExpire)
	case output = <-c:
	}

	return output
}
