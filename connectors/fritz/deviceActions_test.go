package fritz

import (
	"testing"

	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freepslib"
	"gotest.tools/v3/assert"
)

func findAction(actions []*sensor.Action, name string) *sensor.Action {
	for _, a := range actions {
		if a.Name == name {
			return a
		}
	}
	return nil
}

func TestActionsForSwitch(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain1", DeviceID: "42", Switch: &freepslib.AvmDeviceSwitch{}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 3)

	on := findAction(actions, "setswitchon")
	assert.Assert(t, on != nil)
	assert.Equal(t, on.Operator, "fritz")
	assert.Equal(t, on.Fixed["device"], "ain1")
	assert.Equal(t, len(on.Params), 0)

	assert.Assert(t, findAction(actions, "setswitchoff") != nil)
	assert.Assert(t, findAction(actions, "setswitchtoggle") != nil)
	// the device has no simple on/off part, so that command must not show up
	assert.Assert(t, findAction(actions, "setsimpleonoff") == nil)
}

func TestActionsForSimpleOnOff(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain2", DeviceID: "43", SimpleOnOff: &freepslib.AvmDeviceSimpleonoff{}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 1)

	onoff := findAction(actions, "setsimpleonoff")
	assert.Assert(t, onoff != nil)
	assert.Equal(t, len(onoff.Params), 1)
	assert.Equal(t, onoff.Params[0].Name, "onoff")
	assert.Assert(t, onoff.Params[0].Optional)
}

func TestActionsForHKR(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain4", DeviceID: "17",
		HKR: &freepslib.AvmDeviceHkr{Komfort: 36, Absenk: 32}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 1)

	hkr := findAction(actions, "sethkrtsoll")
	assert.Assert(t, hkr != nil)
	// the raw param argument is replaced by a readable temperature
	assert.Equal(t, len(hkr.Params), 1)
	param := hkr.Params[0]
	assert.Equal(t, param.Name, "temperature")
	assert.Equal(t, param.Raw, "param")
	assert.Equal(t, param.Scale, 2.0)
	// the preset labels carry the device's own comfort/eco values
	assert.Equal(t, param.Values["comfort"], "36")
	assert.Equal(t, param.Values["eco"], "32")
	assert.Equal(t, param.Values["off"], "253")
}

func TestActionsForLight(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain5", DeviceID: "2000",
		ColorControl: &freepslib.AvmDeviceColorcontrol{}, LevelControl: &freepslib.AvmDeviceLevelcontrol{}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 3)

	color := findAction(actions, "setcolor")
	assert.Assert(t, color != nil)
	assert.Equal(t, len(color.Params), 3)

	level := findAction(actions, "setlevelpercentage")
	assert.Assert(t, level != nil)
	assert.Equal(t, level.Params[0].Name, "level")

	assert.Assert(t, findAction(actions, "setcolortemperature") != nil)
}

func TestActionsForButton(t *testing.T) {
	dev := freepslib.AvmDevice{DeviceID: "16"} // buttons have no AIN
	assert.Equal(t, len(actionsForDevice("fritz", dev)), 0)
}
