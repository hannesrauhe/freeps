package freepsstore

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type HookStore struct {
	executionLogNs    StoreNamespace
	graphInfoLogNs    StoreNamespace
	operatorInfoLogNs StoreNamespace
	errorLog          *CollectedErrors
	GE                *freepsgraph.GraphEngine
}

var _ freepsgraph.FreepsHook = &HookStore{}

// NewStoreHook creates a new store Hook
func NewStoreHook(cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*HookStore, error) {
	if store.namespaces == nil || store.config == nil {
		return nil, fmt.Errorf("Store was not properly initialized")
	}
	var eLog, glog, olog StoreNamespace
	if store.config.ExecutionLogName != "" {
		eLog = store.GetNamespace(store.config.ExecutionLogName)
	}
	if store.config.GraphInfoName != "" {
		glog = store.GetNamespace(store.config.GraphInfoName)
	}
	if store.config.OperatorInfoName != "" {
		olog = store.GetNamespace(store.config.OperatorInfoName)
	}
	return &HookStore{executionLogNs: eLog, graphInfoLogNs: glog, operatorInfoLogNs: olog, errorLog: NewCollectedErrors(store.config), GE: ge}, nil
}

// GetName returns the name of the hook
func (h *HookStore) GetName() string {
	return "store"
}

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
		return fmt.Errorf("no graph info namespace in hook")
	}
	if graphName == "" {
		return fmt.Errorf("graph name is empty")
	}
	out := h.graphInfoLogNs.UpdateTransaction(graphName, func(oldValue *base.OperatorIO) *base.OperatorIO {
		oldGraphInfo := GraphInfo{}
		newGraphInfo := GraphInfo{ExecutionCounter: 1}
		if mainArgs != nil && len(mainArgs) > 0 {
			newGraphInfo.Arguments = mainArgs
		}
		if mainInput != nil && !mainInput.IsEmpty() {
			newGraphInfo.Input = mainInput.GetString()
		}
		if oldValue != nil {
			oldValue.ParseJSON(&oldGraphInfo)
			newGraphInfo.ExecutionCounter = oldGraphInfo.ExecutionCounter + 1
		}
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
		return fmt.Errorf("no operator info namespace in hook")
	}
	opDetails := ctx.GetOperation(operationIndexInContext)
	out := h.operatorInfoLogNs.UpdateTransaction(opDetails.OpDesc, func(oldValue *base.OperatorIO) *base.OperatorIO {
		fnInfo := FunctionInfo{}
		if oldValue != nil {
			oldValue.ParseJSON(&fnInfo)
		}
		fnInfo.ExecutionCounter++
		fnInfo.LastUsedByGraph = opDetails.GraphName
		return base.MakeObjectOutput(fnInfo)
	}, ctx.GetID())
	if out.IsError() {
		return out.GetError()
	}
	return nil
}

// OnGraphChanged analyzes all graphs and updates the operator info
func (h *HookStore) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	if h.operatorInfoLogNs == nil {
		return fmt.Errorf("no operator info namespace in hook")
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
		out := h.operatorInfoLogNs.UpdateTransaction(opDesc, func(oldValue *base.OperatorIO) *base.OperatorIO {
			fnInfo := FunctionInfo{}
			if oldValue != nil {
				oldValue.ParseJSON(&fnInfo)
			}
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
		return fmt.Errorf("no executionLog namespace in hook")
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

// Shutdown gets called on graceful shutdown
func (h *HookStore) Shutdown() error {
	return nil
}
