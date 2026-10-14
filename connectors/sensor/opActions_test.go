package sensor_test

import (
	"strings"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"gotest.tools/v3/assert"
)

func initActions(t *testing.T) (*sensor.OpSensor, *base.Context) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	return sensor.GetGlobalSensors(), ctx
}

// echoAction targets the utils operator's EchoArguments function, which
// returns the arguments it received, so tests can assert the raw arguments.
func echoAction(name string, fixed map[string]string, params ...sensor.ActionParam) *sensor.Action {
	return &sensor.Action{Name: name, Operator: "Utils", Function: "EchoArguments", Fixed: fixed, Params: params}
}

func execute(op *sensor.OpSensor, ctx *base.Context, actionName string, args map[string]string) *base.OperatorIO {
	return op.ExecuteAction(ctx, base.MakeEmptyOutput(), sensor.ActionArgs{
		DeviceCategory: "act", DeviceName: "dev", ActionName: actionName,
	}, base.NewFunctionArguments(args))
}

func TestExecuteActionScaleAndValues(t *testing.T) {
	op, ctx := initActions(t)
	err := op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("setTargetTemperature", map[string]string{"device": "real"},
			sensor.ActionParam{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
				Values: map[string]string{"off": "253"}}),
	})
	assert.NilError(t, err)

	res := execute(op, ctx, "setTargetTemperature", map[string]string{"temperature": "21.5"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	got := map[string]interface{}{}
	assert.NilError(t, res.ParseJSON(&got))
	assert.Equal(t, got["param"], "43")
	assert.Equal(t, got["device"], "real")

	res = execute(op, ctx, "setTargetTemperature", map[string]string{"temperature": "off"})
	assert.NilError(t, res.ParseJSON(&got))
	assert.Equal(t, got["param"], "253")

	// raw values that are not in the Values map pass through
	res = execute(op, ctx, "setTargetTemperature", map[string]string{"temperature": "254"})
	assert.NilError(t, res.ParseJSON(&got))
	assert.Equal(t, got["param"], "508")
}

func TestExecuteActionFixedWins(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("turnOn", map[string]string{"device": "real"}),
	}))

	res := execute(op, ctx, "turnOn", map[string]string{"device": "evil"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	got := map[string]interface{}{}
	assert.NilError(t, res.ParseJSON(&got))
	assert.Equal(t, got["device"], "real")
}

func TestExecuteActionRejectsBadArgs(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("setLevel", nil, sensor.ActionParam{Name: "level", Type: "int"}),
	}))

	res := execute(op, ctx, "setLevel", map[string]string{"level": "50", "extra": "1"})
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "unknown parameter \"extra\"")

	res = execute(op, ctx, "setLevel", map[string]string{})
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "parameter \"level\" is missing")

	res = execute(op, ctx, "doesNotExist", map[string]string{})
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "available actions")
}

func TestGetActionsAndListing(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "act", "dev", map[string]interface{}{"name": "My Device"}))
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("turnOn", map[string]string{"device": "real"}),
		echoAction("setLevel", nil, sensor.ActionParam{Name: "level", Type: "int", Doc: "percent"}),
	}))

	res := op.GetActions(ctx, base.MakeEmptyOutput(), sensor.DeviceArgs{DeviceCategory: "act", DeviceName: "dev"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	actions := map[string]sensor.ActionInfo{}
	assert.NilError(t, res.ParseJSON(&actions))
	assert.Equal(t, len(actions), 2)
	assert.Equal(t, actions["setLevel"].Params[0].Name, "level")
	// the raw target must not leak into the listing
	assert.Assert(t, !strings.Contains(res.GetString(), "EchoArguments"))

	res = op.GetActionsPerCategory(ctx, base.MakeEmptyOutput())
	perCategory := map[string][]sensor.DeviceActions{}
	assert.NilError(t, res.ParseJSON(&perCategory))
	assert.Equal(t, len(perCategory["act"]), 1)
	assert.Equal(t, perCategory["act"][0].ID, "act.dev")
	assert.Equal(t, perCategory["act"][0].Alias, "My Device")
	assert.Equal(t, len(perCategory["act"][0].Actions), 2)

	res = op.GetActionParams(ctx, base.MakeEmptyOutput(), sensor.ActionArgs{DeviceCategory: "act", DeviceName: "dev", ActionName: "setLevel"})
	params := []base.ArgumentDescription{}
	assert.NilError(t, res.ParseJSON(&params))
	assert.Equal(t, len(params), 1)
	assert.Equal(t, params[0].Name, "level")
	assert.Equal(t, params[0].Type, "int")
}

func TestSetDeviceActionsReplaces(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("turnOn", nil), echoAction("turnOff", nil),
	}))
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("toggle", nil),
	}))

	res := execute(op, ctx, "turnOn", map[string]string{})
	assert.Assert(t, res.IsError())
	res = execute(op, ctx, "toggle", map[string]string{})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())

	assert.NilError(t, op.DeleteActionsInternal("act", "dev"))
	res = execute(op, ctx, "toggle", map[string]string{})
	assert.Assert(t, res.IsError())
}

func TestSetDeviceActionsValidation(t *testing.T) {
	op, _ := initActions(t)
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "de.v", []*sensor.Action{}), "cannot contain a dot")
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		{Name: "", Operator: "Utils", Function: "EchoArguments"},
	}), "empty name")
	assert.ErrorContains(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		{Name: "broken", Operator: "", Function: ""},
	}), "no target")
}

// the free parameters of ExecuteAction get their value suggestions from the selected action,
// which is what the flow editor and the argDetails endpoint ask for.
func TestExecuteActionFreeArgSuggestions(t *testing.T) {
	_, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	op := sensor.GetGlobalSensors()
	assert.NilError(t, op.SetDeviceActionsInternal("act", "dev", []*sensor.Action{
		echoAction("setTargetTemperature", nil,
			sensor.ActionParam{Name: "temperature", Raw: "param", Type: "float", Scale: 2,
				Values: map[string]string{"off": "253", "comfort": "36"}}),
	}))

	sensorOp := ge.GetOperator("sensor")
	otherArgs := base.NewFunctionArguments(map[string]string{
		"deviceCategory": "act", "deviceName": "dev", "actionName": "setTargetTemperature",
	})

	sug := sensorOp.GetArgSuggestions("executeAction", "temperature", otherArgs)
	assert.Equal(t, sug["off"], "off")
	assert.Equal(t, sug["comfort"], "comfort")
	assert.Equal(t, len(sug), 2)

	// an argument that is not a parameter of the action has no suggestions
	assert.Equal(t, len(sensorOp.GetArgSuggestions("executeAction", "nonsense", otherArgs)), 0)
	// without a selected action there is nothing to suggest
	assert.Equal(t, len(sensorOp.GetArgSuggestions("executeAction", "temperature",
		base.NewFunctionArguments(map[string]string{"deviceCategory": "act", "deviceName": "dev"}))), 0)
}
