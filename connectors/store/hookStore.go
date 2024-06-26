package freepsstore

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

type HookStore struct {
	executionLogNs StoreNamespace
	debugNs        StoreNamespace
	GE             *freepsgraph.GraphEngine
}

var _ freepsgraph.FreepsExecutionHook = &HookStore{}
var _ freepsgraph.FreepsGraphChangedHook = &HookStore{}

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
	}, ctx)

	out2 := h.debugNs.SetValue(fmt.Sprintf("GraphInput:%s", graphName), mainInput, ctx)
	out3 := h.debugNs.SetValue(fmt.Sprintf("GraphArguments:%s.", graphName), base.MakeObjectOutput(mainArgs), ctx)
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
func (h *HookStore) OnGraphChanged(ctx *base.Context, addedGraphs []string, removedGraphs []string) error {
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
		}, ctx)
		if out.IsError() {
			return out.GetError()
		}
	}

	for _, graphId := range addedGraphs {
		gd, found := h.GE.GetGraphDesc(graphId)
		if found {
			StoreGraph(fmt.Sprintf("%s.%d", graphId, time.Now().Unix()), *gd, ctx)
		}
	}

	return nil
}

type ExecutionLogEntry struct {
	Input      string
	Output     string
	OutputCode int
	GraphID    string
	Operation  *freepsgraph.GraphOperationDesc
}

// OnExecuteOperation gets called when freepsgraph encounters an error while executing a Graph
func (h *HookStore) OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, graphName string, opDetails *freepsgraph.GraphOperationDesc) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}
	out1 := h.debugNs.UpdateTransaction(fmt.Sprintf("Function:%s.%s", opDetails.Operator, opDetails.Function), func(oldValue base.OperatorIO) *base.OperatorIO {
		fnInfo := FunctionInfo{}
		oldValue.ParseJSON(&fnInfo)
		fnInfo.ExecutionCounter++
		fnInfo.LastUsedByGraph = graphName
		return base.MakeObjectOutput(fnInfo)
	}, ctx)

	out2 := h.debugNs.SetValue(fmt.Sprintf("OperationArguments:%s.%s", graphName, opDetails.Name), base.MakeObjectOutput(opDetails.Arguments), ctx)
	if out1.IsError() {
		return out1.GetError()
	}
	if out2.IsError() {
		return out2.GetError()
	}

	if h.executionLogNs == nil {
		return fmt.Errorf("executionLog namespace missing")
	}
	out := h.executionLogNs.SetValue("", base.MakeObjectOutput(ExecutionLogEntry{Input: input.GetString(), Output: opOutput.GetString(), OutputCode: opOutput.HTTPCode, GraphID: graphName, Operation: opDetails}), ctx)
	if out.IsError() {
		return out.GetError()
	}
	return nil
}

// OnExecutionFinished gets called when freepsgraph is finished executing a Graph
func (h *HookStore) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {

	return nil
}
