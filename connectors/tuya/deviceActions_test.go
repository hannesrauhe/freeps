package tuya

import (
	"testing"

	"github.com/hannesrauhe/freeps/connectors/device"
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

func TestActionsForDevice(t *testing.T) {
	actions := actionsForDevice("tuya", "dehumidifier", DeviceConfig{})

	// DPS 1 is the main switch, so it becomes the canonical turnOn/turnOff
	on := findAction(actions, "turnOn")
	assert.Assert(t, on != nil)
	assert.Equal(t, on.Operator, "tuya")
	assert.Equal(t, on.Function, "setswitch")
	assert.Equal(t, on.Fixed["device"], "dehumidifier")
	assert.Equal(t, on.Fixed["on"], "true")
	assert.Equal(t, len(on.Params), 0)
	assert.Equal(t, findAction(actions, "turnOff").Fixed["on"], "false")

	// DPS 101 is the second switch, which has no canonical name and keeps the generic one
	second := findAction(actions, "setSwitch")
	assert.Assert(t, second != nil)
	assert.Equal(t, second.Fixed["dps"], "101")

	// DPS 2 is the target humidity, so it becomes the canonical setTargetHumidity
	humidity := findAction(actions, "setTargetHumidity")
	assert.Assert(t, humidity != nil)
	assert.Equal(t, humidity.Function, "setdps")
	assert.Equal(t, humidity.Fixed["device"], "dehumidifier")
	assert.Equal(t, humidity.Fixed["dps"], "2")
	assert.Equal(t, len(humidity.Params), 1)
	assert.Equal(t, humidity.Params[0].Name, "humidity")
	assert.Equal(t, humidity.Params[0].Raw, "value")
	assert.Equal(t, humidity.Params[0].Type, "int")

	// DPS 5 workMode is writable, gets the set prefix
	mode := findAction(actions, "setWorkMode")
	assert.Assert(t, mode != nil)
	assert.Equal(t, mode.Fixed["dps"], "5")

	// measured data points are not exposed
	assert.Assert(t, findAction(actions, "setTemperature") == nil)
	assert.Assert(t, findAction(actions, "setFault") == nil)
	assert.Assert(t, findAction(actions, "setFilterLife") == nil)
}

// an explicit name override wins over the canonical mapping
func TestCanonicalActionsOverrideByConfig(t *testing.T) {
	cfg := DeviceConfig{ActionNames: map[string]string{"1": "setSwitch"}}
	actions := actionsForDevice("tuya", "plug", cfg)
	assert.Assert(t, findAction(actions, "setSwitch") != nil)
	assert.Assert(t, findAction(actions, "turnOn") == nil)
}

func TestActionsForDeviceConfigOverride(t *testing.T) {
	cfg := DeviceConfig{ActionNames: map[string]string{
		"5":  "setMode",   // rename
		"4":  "",          // hide
		"23": "setFilter", // explicitly expose a measured point
	}}
	actions := actionsForDevice("tuya", "dev", cfg)
	assert.Assert(t, findAction(actions, "setMode") != nil)
	assert.Assert(t, findAction(actions, "setWorkMode") == nil)
	assert.Assert(t, findAction(actions, "setFanSpeed") == nil)
	assert.Assert(t, findAction(actions, "setFilter") != nil)
}

func TestActionNameForCustomDPS(t *testing.T) {
	cfg := DeviceConfig{DPSNames: map[string]string{"105": "power", "106": "setTimer"}}
	// unknown ids are only exposed when the name marks them as settable
	assert.Equal(t, actionNameFor(cfg, "105", "power"), "")
	assert.Equal(t, actionNameFor(cfg, "106", "setTimer"), "setTimer")
	// an explicit override exposes them regardless
	cfg.ActionNames = map[string]string{"105": "setPower"}
	assert.Equal(t, actionNameFor(cfg, "105", "power"), "setPower")
}
