package flowbuilder_test

import (
	"os"
	"path"
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

	// without tags all flows are listed
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{})
	assert.Assert(t, !out.IsError(), "ListFlows failed: %v", out)
	flows := map[string]freepsflow.FlowDesc{}
	assert.NilError(t, out.ParseJSON(&flows))
	_, exists := flows["listedFlow"]
	assert.Assert(t, exists, "created flow should be listed")
	_, exists = flows["taggedFlow"]
	assert.Assert(t, exists, "tagged flow should be listed")

	// with a tag only the matching flows are listed
	out = fb.ListFlows(ctx, base.MakeEmptyOutput(), flowbuilder.ListFlowsArgs{Tags: strPtr("mytag")})
	assert.Assert(t, !out.IsError(), "ListFlows with tag failed: %v", out)
	flows = map[string]freepsflow.FlowDesc{}
	assert.NilError(t, out.ParseJSON(&flows))
	_, exists = flows["taggedFlow"]
	assert.Assert(t, exists, "tagged flow should be listed for its tag")
	_, exists = flows["listedFlow"]
	assert.Assert(t, !exists, "untagged flow should not be listed for a tag")
}

func strPtr(s string) *string { return &s }
