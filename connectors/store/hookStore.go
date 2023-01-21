package freepsstore

import (
	"fmt"
	"log"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	_ "github.com/lib/pq"
)

type HookStore struct {
	conf  *FreepsStoreConfig
	store StoreNamespace
}

var _ freepsgraph.FreepsHook = &HookStore{}

// NewStoreHook creates a new store Hook
func NewStoreHook(cr *utils.ConfigReader) (*HookStore, error) {
	sc := defaultConfig
	err := cr.ReadSectionWithDefaults("store", &sc)
	if err != nil {
		log.Fatal(err)
	}
	return &HookStore{&sc, store.GetNamespace(sc.ExecutionLogName)}, nil
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
	if h.store == nil {
		return fmt.Errorf("no namespace in hook")
	}
	if !ctx.IsRootContext() {
		return nil
	}
	return h.store.SetValue(ctx.GetID(), freepsgraph.MakeObjectOutput(ctx), ctx.GetID())
}

// Shutdown gets called on graceful shutdown
func (h *HookStore) Shutdown() error {
	return nil
}
