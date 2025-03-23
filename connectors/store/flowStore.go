package freepsstore

import (
	"encoding/json"
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
)

/* this file contains a couple of helper functions to store FlowDesc objects in a separate namespace of the store */

// GetFlowStore returns the flow store
func GetFlowStore() StoreNamespace {
	return store.GetNamespaceNoError("_flows")
}

// GetFlow returns a flow from the store
func GetFlow(name string) (freepsflow.FlowDesc, error) {
	gd := freepsflow.FlowDesc{}
	op := GetFlowStore().GetValue(name)
	if op == NotFoundEntry {
		return gd, fmt.Errorf("Flow \"%s\" not found", name)
	}
	io := op.GetData()
	if io.OutputType != base.Byte {
		return gd, fmt.Errorf("Object \"%s\" is not a serialized Flow", name)
	}
	err := io.ParseJSON(&gd)
	return gd, err
}

// StoreFlow stores a flow in the store
func StoreFlow(name string, gd freepsflow.FlowDesc, modifiedBy *base.Context) *base.OperatorIO {
	b, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return base.MakeOutputError(500, "Failed to marshal flow: %v", err.Error())
	}
	return GetFlowStore().SetValue(name, base.MakeByteOutput(b), modifiedBy).GetData()
}

// DeleteFlow deletes a flow from the store
func DeleteFlow(name string) {
	GetFlowStore().DeleteValue(name)
}
