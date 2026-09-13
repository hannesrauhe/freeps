package flowbuilder_test

import (
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/flowbuilder"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"github.com/hannesrauhe/freeps/freepsflow"
	"gotest.tools/v3/assert"
)

const testFlowJSON = `{"DisplayName":"test flow","Operations":[{"Name":"noopOp","Operator":"utils","Function":"noop","Arguments":{"message":"hello"}}],"OutputFrom":"noopOp"}`

const taggedFlowJSON = `{"DisplayName":"tagged flow","Tags":["mytag"],"Operations":[{"Name":"noopOp","Operator":"utils","Function":"noop"}],"OutputFrom":"noopOp"}`

func boolPtr(b bool) *bool { return &b }

func TestCreateFlow(t *testing.T) {
	ctx, ge, cr := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	out := fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "createdFlow"})
	assert.Assert(t, !out.IsError(), "CreateFlow failed: %v", out)

	gd, exists := ge.GetFlowDesc("createdFlow")
	assert.Assert(t, exists, "flow should exist in engine")
	assert.Equal(t, gd.DisplayName, "test flow")
	assert.Equal(t, len(gd.Operations), 1)

	// the flow must be persisted in the graphs directory
	_, err := os.Stat(path.Join(cr.GetConfigDir(), "graphs", "createdFlow.json"))
	assert.NilError(t, err, "flow file should be written to the graphs directory")

	// and it must be executable
	out = ge.ExecuteFlow(ctx, "createdFlow", base.MakeEmptyFunctionArguments(), base.MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "executing the created flow failed: %v", out)

	// creating it again without overwrite must fail
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "createdFlow"})
	assert.Assert(t, out.IsError(), "creating an existing flow without overwrite should fail")

	// with overwrite it must succeed
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "createdFlow", Overwrite: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "CreateFlow with overwrite failed: %v", out)
}

func TestCreateFlowInvalid(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	// no flowID
	out := fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{})
	assert.Assert(t, out.IsError(), "missing flowID should fail")

	// invalid json
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte("this is not json")), flowbuilder.CreateFlowArgs{FlowID: "invalid"})
	assert.Assert(t, out.IsError(), "invalid json should fail")

	// valid json but no operations
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte(`{"DisplayName":"empty"}`)), flowbuilder.CreateFlowArgs{FlowID: "invalid"})
	assert.Assert(t, out.IsError(), "flow without operations should fail")

	// unknown operator (validation by the engine)
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte(`{"Operations":[{"Operator":"doesNotExist","Function":"nope"}]}`)), flowbuilder.CreateFlowArgs{FlowID: "invalid"})
	assert.Assert(t, out.IsError(), "flow with unknown operator should fail")
}

func TestLiveOperations(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	out := fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "liveFlow"})
	assert.Assert(t, !out.IsError(), "CreateFlow failed: %v", out)

	// add an operation to the flow in the engine
	out = fb.AddOperation(ctx, base.MakeEmptyOutput(), flowbuilder.AddOperation{
		FlowName: "liveFlow", Operator: strPtr("utils"), Function: strPtr("noop"), Live: boolPtr(true),
	})
	assert.Assert(t, !out.IsError(), "AddOperation (live) failed: %v", out)
	gd, _ := ge.GetFlowDesc("liveFlow")
	assert.Equal(t, len(gd.Operations), 2)

	// change the operator of the second operation
	out = fb.SetOperation(ctx, base.MakeEmptyOutput(), flowbuilder.SetOperationArgs{
		FlowName: "liveFlow", OperationNumber: 1, Operator: strPtr("system"), Function: strPtr("noop"), Live: boolPtr(true),
	})
	assert.Assert(t, !out.IsError(), "SetOperation (live) failed: %v", out)
	gd, _ = ge.GetFlowDesc("liveFlow")
	assert.Equal(t, gd.Operations[1].Operator, "system")

	// remove it again
	out = fb.RemoveOperation(ctx, base.MakeEmptyOutput(), flowbuilder.RemoveOperationArgs{FlowName: "liveFlow", OperationNumber: 1, Live: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "RemoveOperation (live) failed: %v", out)
	gd, _ = ge.GetFlowDesc("liveFlow")
	assert.Equal(t, len(gd.Operations), 1)

	// a flow that is not in the engine must give an error
	out = fb.AddOperation(ctx, base.MakeEmptyOutput(), flowbuilder.AddOperation{FlowName: "notThere", Live: boolPtr(true)})
	assert.Assert(t, out.IsError(), "AddOperation on a missing flow should fail")
}

func TestStoreOperationsStillWork(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	// default (no Live) must still work on the draft in the store
	out := fb.GetFlowFromStore(ctx, base.MakeEmptyOutput(), flowbuilder.FlowFromStoreArgs{FlowName: "draftFlow", CreateIfMissing: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "CreateFlowInStore failed: %v", out)

	out = fb.AddOperation(ctx, base.MakeEmptyOutput(), flowbuilder.AddOperation{FlowName: "draftFlow", Operator: strPtr("utils"), Function: strPtr("noop")})
	assert.Assert(t, !out.IsError(), "AddOperation (store) failed: %v", out)

	// the engine must not know the draft
	_, exists := ge.GetFlowDesc("draftFlow")
	assert.Assert(t, !exists, "a store draft must not appear in the engine")
}

func TestPromoteFlow(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	out := fb.GetFlowFromStore(ctx, base.MakeEmptyOutput(), flowbuilder.FlowFromStoreArgs{FlowName: "promoteMe", CreateIfMissing: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "creating draft failed: %v", out)
	out = fb.AddOperation(ctx, base.MakeEmptyOutput(), flowbuilder.AddOperation{FlowName: "promoteMe", Operator: strPtr("utils"), Function: strPtr("noop")})
	assert.Assert(t, !out.IsError(), "AddOperation failed: %v", out)

	out = fb.PromoteFlow(ctx, base.MakeEmptyOutput(), flowbuilder.PromoteFlowArgs{FlowName: "promoteMe"})
	assert.Assert(t, !out.IsError(), "PromoteFlow failed: %v", out)

	gd, exists := ge.GetFlowDesc("promoteMe")
	assert.Assert(t, exists, "promoted flow should be in the engine")
	assert.Equal(t, len(gd.Operations), 1)

	// promoting again without overwrite must fail
	out = fb.PromoteFlow(ctx, base.MakeEmptyOutput(), flowbuilder.PromoteFlowArgs{FlowName: "promoteMe"})
	assert.Assert(t, out.IsError(), "promoting an existing flow without overwrite should fail")

	// promoting a flow that is not in the store must fail
	out = fb.PromoteFlow(ctx, base.MakeEmptyOutput(), flowbuilder.PromoteFlowArgs{FlowName: "notInStore"})
	assert.Assert(t, out.IsError(), "promoting a missing draft should fail")
}

func TestDeleteAndRestore(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	out := fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "deleteMe"})
	assert.Assert(t, !out.IsError(), "CreateFlow failed: %v", out)

	out = fb.DeleteFlow(ctx, base.MakeEmptyOutput(), flowbuilder.FlowFromEngineArgs{FlowID: "deleteMe"})
	assert.Assert(t, !out.IsError(), "DeleteFlow failed: %v", out)
	_, exists := ge.GetFlowDesc("deleteMe")
	assert.Assert(t, !exists, "flow should be gone from the engine")

	// the old restore path must still work via the backup in the store
	out = fb.RestoreDeletedFlowFromStore(ctx, base.MakeEmptyOutput(), flowbuilder.FlowFromStoreArgs{FlowName: "deleteMe"})
	assert.Assert(t, !out.IsError(), "RestoreDeletedFlowFromStore failed: %v", out)
	_, exists = ge.GetFlowDesc("deleteMe")
	assert.Assert(t, exists, "restored flow should be back in the engine")
}

func TestListFlows(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	out := fb.CreateFlow(ctx, base.MakeByteOutput([]byte(testFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "listedFlow"})
	assert.Assert(t, !out.IsError(), "CreateFlow failed: %v", out)
	out = fb.CreateFlow(ctx, base.MakeByteOutput([]byte(taggedFlowJSON)), flowbuilder.CreateFlowArgs{FlowID: "taggedFlow"})
	assert.Assert(t, !out.IsError(), "CreateFlow failed: %v", out)

	// by default all flows are listed, in the brief form without operations
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{})
	assert.Assert(t, !out.IsError(), "ListFlows failed: %v", out)
	brief := map[string]freepsflow.FlowBriefDesc{}
	assert.NilError(t, out.ParseJSON(&brief))
	_, exists := brief["listedFlow"]
	assert.Assert(t, exists, "created flow should be listed")
	_, exists = brief["taggedFlow"]
	assert.Assert(t, exists, "tagged flow should be listed")
	assert.Equal(t, brief["listedFlow"].DisplayName, "test flow")

	flows := map[string]freepsflow.FlowDesc{}
	assert.NilError(t, out.ParseJSON(&flows))
	assert.Equal(t, len(flows["listedFlow"].Operations), 0, "the default listing must not contain operations")

	// with details=true the full definitions are returned
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Details: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "ListFlows with details failed: %v", out)
	flows = map[string]freepsflow.FlowDesc{}
	assert.NilError(t, out.ParseJSON(&flows))
	assert.Equal(t, len(flows["listedFlow"].Operations), 1, "details=true must contain the operations")

	// with a tag only the matching flows are listed
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Tags: strPtr("mytag")})
	assert.Assert(t, !out.IsError(), "ListFlows with tag failed: %v", out)
	brief = map[string]freepsflow.FlowBriefDesc{}
	assert.NilError(t, out.ParseJSON(&brief))
	_, exists = brief["taggedFlow"]
	assert.Assert(t, exists, "tagged flow should be listed for its tag")
	_, exists = brief["listedFlow"]
	assert.Assert(t, !exists, "untagged flow should not be listed for a tag")

	// without a kind all flows count as manual
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Kind: []string{"manual"}})
	assert.Assert(t, !out.IsError(), "ListFlows with kind=manual failed: %v", out)
	brief = map[string]freepsflow.FlowBriefDesc{}
	assert.NilError(t, out.ParseJSON(&brief))
	_, exists = brief["listedFlow"]
	assert.Assert(t, exists, "a flow without kind should be listed for kind=manual")

	// after setting a kind it is only listed for that kind
	out = fb.SetFlowKind(ctx, base.MakeEmptyOutput(), flowbuilder.SetFlowKindArgs{FlowName: "listedFlow", Kind: "helper", Live: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "SetFlowKind failed: %v", out)
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Kind: []string{"manual"}})
	brief = map[string]freepsflow.FlowBriefDesc{}
	assert.NilError(t, out.ParseJSON(&brief))
	_, exists = brief["listedFlow"]
	assert.Assert(t, !exists, "a helper flow should not be listed for kind=manual")
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Kind: []string{"helper"}})
	brief = map[string]freepsflow.FlowBriefDesc{}
	assert.NilError(t, out.ParseJSON(&brief))
	_, exists = brief["listedFlow"]
	assert.Assert(t, exists, "a helper flow should be listed for kind=helper")
	assert.Equal(t, brief["listedFlow"].Kind, "helper", "the brief description should contain the kind")

	// an invalid kind must be rejected
	out = fb.SetFlowKind(ctx, base.MakeEmptyOutput(), flowbuilder.SetFlowKindArgs{FlowName: "listedFlow", Kind: "bogus", Live: boolPtr(true)})
	assert.Assert(t, out.IsError(), "an invalid kind should fail")

	// the description can be set without touching the operations
	out = fb.SetFlowDescription(ctx, base.MakeEmptyOutput(), flowbuilder.SetFlowDescriptionArgs{FlowName: "listedFlow", Description: "a nice flow", Live: boolPtr(true)})
	assert.Assert(t, !out.IsError(), "SetFlowDescription failed: %v", out)
	gd, exists := ge.GetFlowDesc("listedFlow")
	assert.Assert(t, exists)
	assert.Equal(t, gd.Description, "a nice flow")
	assert.Equal(t, gd.Kind, "helper", "setting the description must not change the kind")
	assert.Equal(t, len(gd.Operations), 1, "setting the description must not change the operations")
}

func TestListOperatorsAndFunctions(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	// listOperators returns name and description of all registered operators
	out := fb.ListOperators(ctx, base.MakeEmptyOutput(), flowbuilder.ListOperatorsArgs{})
	assert.Assert(t, !out.IsError(), "ListOperators failed: %v", out)
	ops := []flowbuilder.OperatorDetail{}
	assert.NilError(t, out.ParseJSON(&ops))
	assert.Assert(t, contains(operatorNames(ops), "utils"), "utils operator should be listed, got %v", ops)
	// the description comes from the doc comment of the operator type
	assert.Assert(t, operatorDetails(ops)["Utils"].Description != "", "the utils operator should have a description, got %v", ops)

	// listFunctions returns name and description of the functions of an operator
	out = fb.ListFunctions(ctx, base.MakeEmptyOutput(), flowbuilder.ListFunctionsArgs{Operator: "utils"})
	assert.Assert(t, !out.IsError(), "ListFunctions failed: %v", out)
	fns := []flowbuilder.FunctionDetail{}
	assert.NilError(t, out.ParseJSON(&fns))
	assert.Assert(t, contains(functionNames(fns), "extract"), "extract should be listed for utils, got %v", fns)

	// the description comes from the doc comment of the method
	assert.Assert(t, functionDetails(fns)["Extract"].Description != "", "the extract function should have a description, got %v", fns)

	// the endpoint returns a stable order, whatever order the operator itself uses
	assert.Assert(t, sort.SliceIsSorted(fns, func(i, j int) bool {
		return strings.ToLower(fns[i].Name) < strings.ToLower(fns[j].Name)
	}), "listFunctions should be sorted, got %v", functionNames(fns))

	// an unknown operator is a 404
	out = fb.ListFunctions(ctx, base.MakeEmptyOutput(), flowbuilder.ListFunctionsArgs{Operator: "doesNotExist"})
	assert.Assert(t, out.IsError(), "ListFunctions with an unknown operator should fail")
}

func TestOperatorArgsAndSuggestions(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	// operatorArgs lists the arguments of a function, with type, requiredness and description
	out := fb.OperatorArgs(ctx, base.MakeEmptyOutput(), flowbuilder.OperatorArgsArgs{Operator: "utils", Function: "extract"})
	assert.Assert(t, !out.IsError(), "OperatorArgs failed: %v", out)
	argDescs := []base.ArgumentDescription{}
	assert.NilError(t, out.ParseJSON(&argDescs))
	assert.Equal(t, len(argDescs), 3)
	argNames := []string{}
	for _, a := range argDescs {
		argNames = append(argNames, a.Name)
	}
	assert.Assert(t, contains(argNames, "Type"), "type should be an argument of extract, got %v", argDescs)
	assert.Assert(t, argumentDescriptions(argDescs)["Type"].Description != "", "the type argument should have a description")

	// argDetails returns the suggestions defined by the operator, together with the description
	out = fb.ArgDetails(ctx, base.MakeEmptyOutput(), flowbuilder.ArgDetailsArgs{Operator: "utils", Function: "extract"})
	assert.Assert(t, !out.IsError(), "ArgDetails failed: %v", out)
	details := []flowbuilder.ArgDetail{}
	assert.NilError(t, out.ParseJSON(&details))
	assert.Equal(t, len(details), 3)
	byName := map[string]flowbuilder.ArgDetail{}
	for _, d := range details {
		byName[d.Name] = d
	}
	_, exists := byName["Type"].Suggestions["string"]
	assert.Assert(t, exists, "string should be a suggestion for the type argument, got %v", byName["Type"].Suggestions)
	assert.Assert(t, byName["Type"].Description != "", "the type argument should have a description")
	assert.Equal(t, byName["Type"].Required, false)
	assert.Equal(t, byName["Type"].Type, "string")
	assert.Equal(t, byName["Key"].Required, true)

	// an unknown operator is a 404
	out = fb.ArgDetails(ctx, base.MakeEmptyOutput(), flowbuilder.ArgDetailsArgs{Operator: "doesNotExist", Function: "extract"})
	assert.Assert(t, out.IsError(), "ArgDetails with an unknown operator should fail")

	// invalid otherArgs is a 400
	out = fb.ArgDetails(ctx, base.MakeEmptyOutput(), flowbuilder.ArgDetailsArgs{Operator: "utils", Function: "extract", OtherArgs: strPtr("%zz")})
	assert.Assert(t, out.IsError(), "ArgDetails with invalid otherArgs should fail")
}

func TestArgDetailsContextSensitive(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	fb := &flowbuilder.OpFlowBuilder{GE: ge}

	// write a value so that the store has a namespace and a key to suggest
	out := ge.ExecuteOperatorByName(ctx, "store", "setSimpleValue", base.NewFunctionArguments(map[string]string{"namespace": "testing", "key": "testkey", "value": "testvalue"}), base.MakeEmptyOutput())
	assert.Assert(t, !out.IsError(), "setSimpleValue failed: %v", out)

	// otherArgs are passed to the suggestion functions, so they can be context sensitive
	out = fb.ArgDetails(ctx, base.MakeEmptyOutput(), flowbuilder.ArgDetailsArgs{Operator: "store", Function: "get", OtherArgs: strPtr("namespace=testing")})
	assert.Assert(t, !out.IsError(), "ArgDetails failed: %v", out)
	details := []flowbuilder.ArgDetail{}
	assert.NilError(t, out.ParseJSON(&details))
	byName := map[string]flowbuilder.ArgDetail{}
	for _, d := range details {
		byName[d.Name] = d
	}
	// the namespace argument has the store namespaces as suggestions
	assert.Assert(t, len(byName["Namespace"].Suggestions) > 0, "the namespace argument should have suggestions, got %v", byName["Namespace"].Suggestions)
	// the doc tags of the store args struct provide descriptions
	assert.Assert(t, byName["Key"].Description != "", "the key argument should have a description")
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

func operatorNames(list []flowbuilder.OperatorDetail) []string {
	names := []string{}
	for _, o := range list {
		names = append(names, o.Name)
	}
	return names
}

func operatorDetails(list []flowbuilder.OperatorDetail) map[string]flowbuilder.OperatorDetail {
	byName := map[string]flowbuilder.OperatorDetail{}
	for _, o := range list {
		byName[o.Name] = o
	}
	return byName
}

func functionNames(list []flowbuilder.FunctionDetail) []string {
	names := []string{}
	for _, f := range list {
		names = append(names, f.Name)
	}
	return names
}

func functionDetails(list []flowbuilder.FunctionDetail) map[string]flowbuilder.FunctionDetail {
	byName := map[string]flowbuilder.FunctionDetail{}
	for _, f := range list {
		byName[f.Name] = f
	}
	return byName
}

func argumentDescriptions(list []base.ArgumentDescription) map[string]base.ArgumentDescription {
	byName := map[string]base.ArgumentDescription{}
	for _, a := range list {
		byName[a.Name] = a
	}
	return byName
}

func strPtr(s string) *string { return &s }
