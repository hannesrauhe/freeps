package freepsstore

import (
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	_ "github.com/lib/pq"
)

type HookStore struct {
}

var _ freepsgraph.FreepsHook = &HookStore{}

// NewStoreHook creates a new store Hook
func NewStoreHook(cr *utils.ConfigReader) (*HookStore, error) {
	return &HookStore{}, nil
}

// GetName returns the name of the hook
func (h *HookStore) GetName() string {
	return "store"
}

// OnExecute gets called when freepsgraph starts executing a Graph
func (h *HookStore) OnExecute(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	return nil
}

// OnExecutionFinished gets called when freepsgraph starts executing a Graph
func (h *HookStore) OnExecutionFinished(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *freepsgraph.OperatorIO) error {
	nsStore := store.GetNamespace("_context")
	nsStore.SetValue(ctx.GetID(), freepsgraph.MakeObjectOutput(ctx), ctx.GetID())
	return nil
}

// Shutdown gets called on graceful shutdown
func (h *HookStore) Shutdown() error {
	return nil
}
