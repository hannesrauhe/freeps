package freepsstore

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookStore struct {
	executionLogNs    StoreNamespace
	graphInfoLogNs    StoreNamespace
	operatorInfoLogNs StoreNamespace
	errorLog          *CollectedErrors
	GE                *freepsgraph.GraphEngine
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
	if h.graphInfoLogNs == nil {
		return fmt.Errorf("graph info namespace missing")
	}
	if graphName == "" {
		return fmt.Errorf("graph name is empty")
	}
	out := h.graphInfoLogNs.UpdateTransaction(graphName, func(oldValue base.OperatorIO) *base.OperatorIO {
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
	if out.IsError() {
		return out.GetError()
	}
	return nil
}

// OnExecuteOperation gets called when freepsgraph starts executing an Operation
func (h *HookStore) OnExecuteOperation(ctx *base.Context, operationIndexInContext int) error {
	if h.operatorInfoLogNs == nil {
		return fmt.Errorf("operator info namespace missing")
	}
	opDetails := ctx.GetOperation(operationIndexInContext)
	out1 := h.operatorInfoLogNs.UpdateTransaction(opDetails.OpDesc, func(oldValue base.OperatorIO) *base.OperatorIO {
		fnInfo := FunctionInfo{}
		oldValue.ParseJSON(&fnInfo)
		fnInfo.ExecutionCounter++
		fnInfo.LastUsedByGraph = opDetails.GraphName
		return base.MakeObjectOutput(fnInfo)
	}, ctx.GetID())

	out2 := h.graphInfoLogNs.SetValue(fmt.Sprintf("%s.%s.Arguments", opDetails.GraphName, opDetails.OpName), base.MakeObjectOutput(opDetails.Arguments), ctx.GetID())
	out3 := h.graphInfoLogNs.SetValue(fmt.Sprintf("%s.%s.ExecutionDuration", opDetails.GraphName, opDetails.OpName), base.MakeObjectOutput(opDetails.ExecutionDuration), ctx.GetID())
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
	if h.operatorInfoLogNs == nil {
		return fmt.Errorf("operator info namespace missing")
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
		out := h.operatorInfoLogNs.UpdateTransaction(opDesc, func(oldValue base.OperatorIO) *base.OperatorIO {
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
