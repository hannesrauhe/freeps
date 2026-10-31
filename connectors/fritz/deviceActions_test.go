package fritz

import (
	"testing"

	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freepslib"
	"gotest.tools/v3/assert"
)

func findAction(actions []*device.Action, name string) *device.Action {
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

	on := findAction(actions, "turnOn")
	assert.Assert(t, on != nil)
	assert.Equal(t, on.Operator, "fritz")
	assert.Equal(t, on.Function, "setswitchon")
	assert.Equal(t, on.Fixed["device"], "ain1")
	assert.Assert(t, on.Param == nil)

	assert.Equal(t, findAction(actions, "turnOff").Function, "setswitchoff")
	assert.Equal(t, findAction(actions, "toggle").Function, "setswitchtoggle")
}

func TestActionsForSimpleOnOff(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain2", DeviceID: "43", SimpleOnOff: &freepslib.AvmDeviceSimpleonoff{}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 2)

	// the on/off part is stateless, so the state goes in as a fixed raw argument
	on := findAction(actions, "turnOn")
	assert.Assert(t, on != nil)
	assert.Equal(t, on.Function, "setsimpleonoff")
	assert.Equal(t, on.Fixed["onoff"], "1")
	assert.Equal(t, findAction(actions, "turnOff").Fixed["onoff"], "0")
	// the on/off part cannot toggle
	assert.Assert(t, findAction(actions, "toggle") == nil)
}

// a device with both parts must not list turnOn twice, the first capability wins
func TestActionsForSwitchAndSimpleOnOff(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain3", DeviceID: "25",
		SimpleOnOff: &freepslib.AvmDeviceSimpleonoff{}, Switch: &freepslib.AvmDeviceSwitch{}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 3)
	assert.Equal(t, findAction(actions, "turnOn").Function, "setsimpleonoff")
	assert.Equal(t, findAction(actions, "toggle").Function, "setswitchtoggle")
}

func TestActionsForHKR(t *testing.T) {
	dev := freepslib.AvmDevice{AIN: "ain4", DeviceID: "17",
		HKR: &freepslib.AvmDeviceHkr{Komfort: 36, Absenk: 32}}
	actions := actionsForDevice("fritz", dev)
	assert.Equal(t, len(actions), 1)

	hkr := findAction(actions, "setTargetTemperature")
	assert.Assert(t, hkr != nil)
	assert.Equal(t, hkr.Function, "sethkrtsoll")
	// the raw param argument is replaced by a readable temperature
	assert.Assert(t, hkr.Param != nil)
	param := *hkr.Param
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
	assert.Equal(t, len(actions), 2)

	// the raw hue/saturation color command has no canonical equivalent and stays hidden
	assert.Assert(t, findAction(actions, "setColor") == nil)

	brightness := findAction(actions, "setBrightness")
	assert.Assert(t, brightness != nil)
	assert.Equal(t, brightness.Function, "setlevelpercentage")
	assert.Equal(t, brightness.Param.Name, "brightness")
	assert.Equal(t, brightness.Param.Raw, "level")
	assert.Equal(t, brightness.Param.Type, "int")

	colorTemp := findAction(actions, "setColorTemperature")
	assert.Assert(t, colorTemp != nil)
	assert.Equal(t, colorTemp.Function, "setcolortemperature")
	assert.Equal(t, colorTemp.Param.Type, "int")
}

func TestActionsForButton(t *testing.T) {
	dev := freepslib.AvmDevice{DeviceID: "16"} // buttons have no AIN
	assert.Equal(t, len(actionsForDevice("fritz", dev)), 0)
}
