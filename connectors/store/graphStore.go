package freepsstore

import (
	"encoding/json"
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

/* this file contains a couple of helper functions to store GraphDesc objects in a separate namespace of the store */

// GetGraphStore returns the graph store
func GetGraphStore() StoreNamespace {
	return store.GetNamespace("_graphs")
}

// GetGraph returns a graph from the store
func GetGraph(name string) (freepsgraph.GraphDesc, error) {
	gd := freepsgraph.GraphDesc{}
	op := GetGraphStore().GetValue(name)
	if op == NotFoundEntry {
		return gd, fmt.Errorf("Graph \"%s\" not found", name)
	}
	io := op.GetData()
	if io.OutputType != base.Byte {
		return gd, fmt.Errorf("Object \"%s\" is not a serialized Graph", name)
	}
	err := io.ParseJSON(&gd)
	return gd, err
}

// StoreGraph stores a graph in the store
func StoreGraph(name string, gd freepsgraph.GraphDesc, modifiedBy string) *base.OperatorIO {
	b, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return base.MakeOutputError(500, "Failed to marshal graph: "+err.Error())
	}
	return GetGraphStore().SetValue(name, base.MakeByteOutput(b), modifiedBy)
}

// DeleteGraph deletes a graph from the store
func DeleteGraph(name string) {
	GetGraphStore().DeleteValue(name)
}
