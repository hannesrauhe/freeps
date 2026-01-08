package freepsstore

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
)

type HookStore struct {
	executionLogNs StoreNamespace
	debugNs        StoreNamespace
	GE             *freepsflow.FlowEngine
}

var _ freepsflow.FreepsExecutionHook = &HookStore{}
var _ freepsflow.FreepsFlowChangedHook = &HookStore{}

// FlowInfo keeps information about a flow execution
type FlowInfo struct {
	ExecutionCounter uint64
	Arguments        map[string]string `json:",omitempty"`
	Input            string            `json:",omitempty"`
}

// FunctionInfo keeps information about the usage of functions
type FunctionInfo struct {
	ExecutionCounter uint64
	ReferenceCounter uint64
	LastUsedByFlow   string `json:",omitempty"`
}

// OnExecute gets called when freepsflow starts executing a Flow
func (h *HookStore) OnExecute(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}
	if flowName == "" {
		return fmt.Errorf("flow name is empty")
	}
	out1 := h.debugNs.UpdateTransaction(fmt.Sprintf("FlowInfo:%s", flowName), func(oldValue StoreEntry) *base.OperatorIO {
		oldFlowInfo := FlowInfo{}
		newFlowInfo := FlowInfo{ExecutionCounter: 1}
		if mainArgs != nil && len(mainArgs) > 0 {
			newFlowInfo.Arguments = mainArgs
		}
		if mainInput != nil && !mainInput.IsEmpty() {
			newFlowInfo.Input = mainInput.GetString()
		}
		oldValue.ParseJSON(&oldFlowInfo)
		newFlowInfo.ExecutionCounter = oldFlowInfo.ExecutionCounter + 1
		return base.MakeObjectOutput(newFlowInfo)
	}, ctx)

	out2 := h.debugNs.SetValue(fmt.Sprintf("FlowInput:%s", flowName), mainInput, ctx)
	out3 := h.debugNs.SetValue(fmt.Sprintf("FlowArguments:%s.", flowName), base.MakeObjectOutput(mainArgs), ctx)
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

// OnFlowChanged analyzes all flows and updates the operator info
func (h *HookStore) OnFlowChanged(ctx *base.Context, addedFlows []string, removedFlows []string) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}

	collectedInfo := map[string]FunctionInfo{}
	for flowName, gd := range h.GE.GetAllFlowDesc() {
		for _, op := range gd.Operations {
			opDesc := fmt.Sprintf("%v.%v", op.Operator, op.Function)
			fnInfo := FunctionInfo{}
			fnInfo, _ = collectedInfo[opDesc]
			fnInfo.ReferenceCounter++
			fnInfo.LastUsedByFlow = flowName
			collectedInfo[opDesc] = fnInfo
		}
	}

	for opDesc, newInfo := range collectedInfo {
		out := h.debugNs.UpdateTransaction(fmt.Sprintf("Function:%s", opDesc), func(oldValue StoreEntry) *base.OperatorIO {
			fnInfo := FunctionInfo{}
			oldValue.ParseJSON(&fnInfo)
			fnInfo.ReferenceCounter = newInfo.ReferenceCounter
			if fnInfo.LastUsedByFlow == "" {
				fnInfo.LastUsedByFlow = newInfo.LastUsedByFlow
			}
			return base.MakeObjectOutput(fnInfo)
		}, ctx)
		if out.IsError() {
			return out.GetError()
		}
	}

	for _, flowId := range addedFlows {
		gd, found := h.GE.GetFlowDesc(flowId)
		if found {
			StoreFlow(fmt.Sprintf("%s.%d", flowId, time.Now().Unix()), *gd, ctx)
		}
	}

	return nil
}

type ExecutionLogEntry struct {
	Input      string
	Output     string
	OutputCode int
	FlowID     string
	Operation  *freepsflow.FlowOperationDesc
}

// OnExecuteOperation gets called when freepsflow encounters an error while executing a Flow
func (h *HookStore) OnExecuteOperation(ctx *base.Context, input *base.OperatorIO, opOutput *base.OperatorIO, flowName string, opDetails *freepsflow.FlowOperationDesc) error {
	if h.debugNs == nil {
		return fmt.Errorf("missing debug namespace")
	}
	out1 := h.debugNs.UpdateTransaction(fmt.Sprintf("Function:%s.%s", opDetails.Operator, opDetails.Function), func(oldValue StoreEntry) *base.OperatorIO {
		fnInfo := FunctionInfo{}
		oldValue.ParseJSON(&fnInfo)
		fnInfo.ExecutionCounter++
		fnInfo.LastUsedByFlow = flowName
		return base.MakeObjectOutput(fnInfo)
	}, ctx)

	out2 := h.debugNs.SetValue(fmt.Sprintf("OperationArguments:%s.%s", flowName, opDetails.Name), base.MakeObjectOutput(opDetails.FunctionArgs), ctx)
	if out1.IsError() {
		return out1.GetError()
	}
	if out2.IsError() {
		return out2.GetError()
	}

	if h.executionLogNs == nil {
		return fmt.Errorf("executionLog namespace missing")
	}
	out := h.executionLogNs.SetValue("", base.MakeObjectOutput(ExecutionLogEntry{Input: input.GetString(), Output: opOutput.GetString(), OutputCode: opOutput.HTTPCode, FlowID: flowName, Operation: opDetails}), ctx)
	if out.IsError() {
		return out.GetError()
	}
	return nil
}

// OnExecutionFinished gets called when freepsflow is finished executing a Flow
func (h *HookStore) OnExecutionFinished(ctx *base.Context, flowName string, mainArgs map[string]string, mainInput *base.OperatorIO) error {

	return nil
}
