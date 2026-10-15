package pixeldisplay

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"gotest.tools/v3/assert"
)

func TestActionsForDisplay(t *testing.T) {
	actions := actionsForDisplay("pixeldisplay")
	byName := map[string]*device.Action{}
	for _, a := range actions {
		byName[a.Name] = a
	}
	assert.Equal(t, len(byName), 5)

	brightness := byName["setBrightness"]
	assert.Assert(t, brightness != nil)
	assert.Equal(t, brightness.Operator, "pixeldisplay")
	assert.Equal(t, brightness.Function, "SetBrightness")
	assert.Equal(t, len(brightness.Params), 1)
	assert.Equal(t, brightness.Params[0].Name, "brightness")

	assert.Equal(t, byName["turnOn"].Function, "TurnOn")
	assert.Equal(t, len(byName["turnOn"].Params), 0)
}

func TestDeviceCategory(t *testing.T) {
	assert.Equal(t, (&OpPixelDisplay{name: "pixeldisplay"}).deviceCategory(), "pixeldisplay")
	assert.Equal(t, (&OpPixelDisplay{name: "pixeldisplay.matrix"}).deviceCategory(), "pixeldisplay_matrix")
}

func TestRegisterDevice(t *testing.T) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	opSensor := device.GetGlobalSensors()

	op := &OpPixelDisplay{name: "pixeldisplay"}
	op.StartListening(ctx)

	res := opSensor.GetActions(ctx, base.MakeEmptyOutput(), device.DeviceArgs{DeviceCategory: "pixeldisplay", DeviceName: "display"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	actions := map[string]device.ActionInfo{}
	assert.NilError(t, res.ParseJSON(&actions))
	assert.Equal(t, len(actions), 5)
	_, hasBrightness := actions["setBrightness"]
	assert.Assert(t, hasBrightness)

	alias := opSensor.GetSensorAlias(ctx, base.MakeEmptyOutput(), device.SensorArgs{SensorCategory: "pixeldisplay", SensorName: "display"})
	assert.Equal(t, alias.GetString(), "Pixel Display")
}
