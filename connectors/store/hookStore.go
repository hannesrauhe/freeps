package freepsstore

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type HookStore struct {
	storeNs StoreNamespace
}

var _ freepsgraph.FreepsHook = &HookStore{}

// NewStoreHook creates a new store Hook
func NewStoreHook(cr *utils.ConfigReader) (*HookStore, error) {
	if store.namespaces == nil || store.config == nil {
		return nil, fmt.Errorf("Store was not properly initialized")
	}
	return &HookStore{store.GetNamespace(store.config.ExecutionLogName)}, nil
}

// GetName returns the name of the hook
func (h *HookStore) GetName() string {
	return "store"
}

// OnExecute gets called when freepsgraph starts executing a Graph
func (h *HookStore) OnExecute(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnExecutionFinished gets called when freepsgraph starts executing a Graph
func (h *HookStore) OnExecutionFinished(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	if h.storeNs == nil {
		return fmt.Errorf("no namespace in hook")
	}
	if !ctx.IsRootContext() {
		return nil
	}
	return h.storeNs.SetValue(ctx.GetID(), freepsgraph.MakeObjectOutput(ctx), ctx.GetID())
}

// OnGraphChanged does nothing in store
func (h *HookStore) OnGraphChanged(addedGraphName []string, removedGraphName []string) error {
	return nil
}

// Shutdown gets called on graceful shutdown
func (h *HookStore) Shutdown() error {
	return nil
}
