package freepsstore

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookStore struct {
	executionLogNs StoreNamespace
	debugNs        StoreNamespace
	errorLog       *CollectedErrors
	GE             *freepsgraph.GraphEngine
}

var _ freepsgraph.FreepsHook = &HookStore{}

// GraphInfo keeps information about a graph execution
type GraphInfo struct {
	ExecutionCounter uint64
	Arguments        map[string]string `json:",omitempty"`
	Input            string            `json:",omitempty"`
}

// FunctionInfo keeps information about the usage of functions
type FunctionInfo struct {
	ExecutionCounter uint64
	ReferenceCounter uint64
	LastUsedByGraph  string `json:",omitempty"`
}

// OnExecute gets called when freepsgraph starts executing a Graph
func (h *HookStore) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}
	if graphName == "" {
		return fmt.Errorf("graph name is empty")
	}
	out1 := h.debugNs.UpdateTransaction(fmt.Sprintf("GraphInfo:%s", graphName), func(oldValue base.OperatorIO) *base.OperatorIO {
		oldGraphInfo := GraphInfo{}
		newGraphInfo := GraphInfo{ExecutionCounter: 1}
		if mainArgs != nil && len(mainArgs) > 0 {
			newGraphInfo.Arguments = mainArgs
		}
		if mainInput != nil && !mainInput.IsEmpty() {
			newGraphInfo.Input = mainInput.GetString()
		}
		oldValue.ParseJSON(&oldGraphInfo)
		newGraphInfo.ExecutionCounter = oldGraphInfo.ExecutionCounter + 1
		return base.MakeObjectOutput(newGraphInfo)
	}, ctx.GetID())

	out2 := h.debugNs.SetValue(fmt.Sprintf("GraphInput:%s", graphName), mainInput, ctx.GetID())
	out3 := h.debugNs.SetValue(fmt.Sprintf("GraphArguments:%s.", graphName), base.MakeObjectOutput(mainArgs), ctx.GetID())
	if out1.IsError() {
		return out1.GetError()
	}
	if out2.IsError() {
		return out2.GetError()
	}
	if out3.IsError() {
		return out3.GetError()
	}
	return nil
}

// OnExecuteOperation gets called when freepsgraph starts executing an Operation
func (h *HookStore) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}
	opDetails := ctx.GetOperation(operationIndexInContext)
	out1 := h.debugNs.UpdateTransaction(fmt.Sprintf("Function:%s", opDetails.OpDesc), func(oldValue base.OperatorIO) *base.OperatorIO {
		fnInfo := FunctionInfo{}
		oldValue.ParseJSON(&fnInfo)
		fnInfo.ExecutionCounter++
		fnInfo.LastUsedByGraph = opDetails.GraphName
		return base.MakeObjectOutput(fnInfo)
	}, ctx.GetID())

	out2 := h.debugNs.SetValue(fmt.Sprintf("OperationArguments:%s.%s", opDetails.GraphName, opDetails.OpName), base.MakeObjectOutput(opDetails.Arguments), ctx.GetID())
	out3 := h.debugNs.SetValue(fmt.Sprintf("OperationDuration:%s.%s.", opDetails.GraphName, opDetails.OpName), base.MakeObjectOutput(opDetails.ExecutionDuration), ctx.GetID())
	if out1.IsError() {
		return out1.GetError()
	}
	if out2.IsError() {
		return out2.GetError()
	}
	if out3.IsError() {
		return out3.GetError()
	}
	return nil
}

// OnGraphChanged analyzes all graphs and updates the operator info
func (h *HookStore) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}

	collectedInfo := map[string]FunctionInfo{}
	for graphName, gd := range h.GE.GetAllGraphDesc() {
		for _, op := range gd.Operations {
			opDesc := fmt.Sprintf("%v.%v", op.Operator, op.Function)
			fnInfo := FunctionInfo{}
			fnInfo, _ = collectedInfo[opDesc]
			fnInfo.ReferenceCounter++
			fnInfo.LastUsedByGraph = graphName
			collectedInfo[opDesc] = fnInfo
		}
	}

	for opDesc, newInfo := range collectedInfo {
		out := h.debugNs.UpdateTransaction(fmt.Sprintf("Function:%s", opDesc), func(oldValue base.OperatorIO) *base.OperatorIO {
			fnInfo := FunctionInfo{}
			oldValue.ParseJSON(&fnInfo)
			fnInfo.ReferenceCounter = newInfo.ReferenceCounter
			if fnInfo.LastUsedByGraph == "" {
				fnInfo.LastUsedByGraph = newInfo.LastUsedByGraph
			}
			return base.MakeObjectOutput(fnInfo)
		}, "")
		if out.IsError() {
			return out.GetError()
		}
	}

	return nil
}

// OnExecutionError gets called when freepsgraph encounters an error while executing a Graph
func (h *HookStore) OnExecutionError(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *freepsgraph.GraphOperationDesc) error {
	return h.errorLog.AddError(input, err, ctx, graphName, od)
}

// OnExecutionFinished gets called when freepsgraph is finished executing a Graph
func (h *HookStore) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	if h.executionLogNs == nil {
		return fmt.Errorf("executionLog namespace missing")
	}
	if !ctx.IsRootContext() {
		return nil
	}
	out := h.executionLogNs.SetValue(ctx.GetID(), base.MakeObjectOutput(ctx), ctx.GetID()).GetData()
	if out.IsError() {
		return out.GetError()
	}
	return nil
}
