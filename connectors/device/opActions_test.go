package device_test

import (
	"strings"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"gotest.tools/v3/assert"
)

func initActions(t *testing.T) (*device.OpDevice, *base.Context) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	return device.GetGlobalSensors(), ctx
}

// echoAction targets the utils operator's EchoArguments function, which
// returns the arguments it received, so tests can assert the raw arguments.
func echoAction(name string, fixed map[string]string, param *device.ActionParam) *device.Action {
	return &device.Action{Name: name, Operator: "Utils", Function: "EchoArguments", Fixed: fixed, Param: param}
}

func execute(op *device.OpDevice, ctx *base.Context, actionName string, value string) *base.OperatorIO {
	return op.ExecuteAction(ctx, base.MakeEmptyOutput(), device.ActionArgs{
		DeviceCategory: "act", DeviceName: "dev", ActionName: actionName, Value: value,
	})
}

func TestExecuteActionSucceeds(t *testing.T) {
	op, ctx := initActions(t)
	err := op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("setTargetTemperature", map[string]string{"device": "real"},
			&device.ActionParam{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
				Values: map[string]string{"off": "253"}}),
	})
	assert.NilError(t, err)

	// a successful action returns an empty success, not the operator's raw output
	res := execute(op, ctx, "setTargetTemperature", "21.5")
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	assert.Assert(t, res.IsEmpty())
}

func TestExecuteActionRejectsBadArgs(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("setLevel", nil, &device.ActionParam{Name: "level", Type: "int", Scale: 2}),
	}))

	res := execute(op, ctx, "setLevel", "not a number")
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "invalid")

	res = execute(op, ctx, "setLevel", "")
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "needs a level")

	res = execute(op, ctx, "doesNotExist", "")
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "available actions")
}

func TestGetActionsAndListing(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "act", "dev", map[string]interface{}{"name": "My Device"}))
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("turnOn", map[string]string{"device": "real"}, nil),
		echoAction("setLevel", nil, &device.ActionParam{Name: "level", Type: "int", Doc: "percent"}),
	}))

	res := op.GetActions(ctx, base.MakeEmptyOutput(), device.DeviceArgs{DeviceCategory: "act", DeviceName: "dev"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	actions := map[string]device.ActionInfo{}
	assert.NilError(t, res.ParseJSON(&actions))
	assert.Equal(t, len(actions), 2)
	assert.Assert(t, actions["turnOn"].Param == nil)
	assert.Equal(t, actions["setLevel"].Param.Name, "level")
	// the raw target must not leak into the listing
	assert.Assert(t, !strings.Contains(res.GetString(), "EchoArguments"))

	res = op.GetActionsPerCategory(ctx, base.MakeEmptyOutput())
	perCategory := map[string][]device.DeviceActions{}
	assert.NilError(t, res.ParseJSON(&perCategory))
	assert.Equal(t, len(perCategory["act"]), 1)
	assert.Equal(t, perCategory["act"][0].ID, "act.dev")
	assert.Equal(t, perCategory["act"][0].Alias, "My Device")
	assert.Equal(t, len(perCategory["act"][0].Actions), 2)

	res = op.GetActionParams(ctx, base.MakeEmptyOutput(), device.ActionArgs{DeviceCategory: "act", DeviceName: "dev", ActionName: "setLevel"})
	params := []base.ArgumentDescription{}
	assert.NilError(t, res.ParseJSON(&params))
	assert.Equal(t, len(params), 1)
	assert.Equal(t, params[0].Name, "level")
	assert.Equal(t, params[0].Type, "int")
}

func TestSetDeviceActionsReplaces(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("turnOn", nil, nil), echoAction("turnOff", nil, nil),
	}))
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("toggle", nil, nil),
	}))

	res := execute(op, ctx, "turnOn", "")
	assert.Assert(t, res.IsError())
	res = execute(op, ctx, "toggle", "")
	assert.Assert(t, !res.IsError(), "%v", res.GetError())

	assert.NilError(t, op.DeleteActionsInternal("act", "dev"))
	res = execute(op, ctx, "toggle", "")
	assert.Assert(t, res.IsError())
}

func TestSetDeviceActionsValidation(t *testing.T) {
	op, _ := initActions(t)
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "de.v", []*device.Action{}), "cannot contain a dot")
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		{Name: "", Operator: "Utils", Function: "EchoArguments"},
	}), "empty name")
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		{Name: "broken", Operator: "", Function: ""},
	}), "no target")
}

// the value argument of ExecuteAction gets its suggestions from the selected action,
// which is what the flow editor and the argDetails endpoint ask for.
func TestExecuteActionValueSuggestions(t *testing.T) {
	_, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	op := device.GetGlobalSensors()
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("setTargetTemperature", nil,
			&device.ActionParam{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
				Values: map[string]string{"off": "253", "comfort": "36"}}),
	}))

	sensorOp := ge.GetOperator("device")
	otherArgs := base.NewFunctionArguments(map[string]string{
		"deviceCategory": "act", "deviceName": "dev", "actionName": "setTargetTemperature",
	})

	sug := sensorOp.GetArgSuggestions("executeAction", "value", otherArgs)
	assert.Equal(t, sug["off"], "off")
	assert.Equal(t, sug["comfort"], "comfort")

	// an action without a parameter has nothing to suggest
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*device.Action{
		echoAction("turnOn", nil, nil),
	}))
	assert.Equal(t, len(sensorOp.GetArgSuggestions("executeAction", "value",
		base.NewFunctionArguments(map[string]string{"deviceCategory": "act", "deviceName": "dev", "actionName": "turnOn"}))), 0)
	// without a selected action there is nothing to suggest either
	assert.Equal(t, len(sensorOp.GetArgSuggestions("executeAction", "value",
		base.NewFunctionArguments(map[string]string{"deviceCategory": "act", "deviceName": "dev"}))), 0)
}
