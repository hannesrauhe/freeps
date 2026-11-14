//go:build !notuya

package tuya

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"github.com/sirupsen/logrus"
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
	actions := actionsForDevice("tuya", "dehumidifier", DeviceConfig{}, nil)

	// DPS 1 is the main switch, so it becomes the canonical turnOn/turnOff
	on := findAction(actions, "turnOn")
	assert.Assert(t, on != nil)
	assert.Equal(t, on.Operator, "tuya")
	assert.Equal(t, on.Function, "setswitch")
	assert.Equal(t, on.Fixed["device"], "dehumidifier")
	assert.Equal(t, on.Fixed["on"], "true")
	assert.Assert(t, on.Param == nil)
	assert.Equal(t, findAction(actions, "turnOff").Fixed["on"], "false")

	// DPS 101 is the second switch, which has no canonical name and keeps the generic one
	second := findAction(actions, "setSwitch")
	assert.Assert(t, second != nil)
	assert.Equal(t, second.Fixed["dps"], "101")

	// DPS 2 is the target humidity, so it becomes the canonical setTargetHumidity
	humidity := findAction(actions, "setTargetHumidity")
	assert.Assert(t, humidity != nil)
	assert.Equal(t, humidity.Function, "setdps")
	assert.Equal(t, humidity.Fixed["dps"], "2")
	assert.Assert(t, humidity.Param != nil)
	assert.Equal(t, humidity.Param.Name, "humidity")
	assert.Equal(t, humidity.Param.Raw, "value")
	assert.Equal(t, humidity.Param.Type, "int")

	// DPS 5 workMode is writable, gets the set prefix
	mode := findAction(actions, "setWorkMode")
	assert.Assert(t, mode != nil)
	assert.Equal(t, mode.Fixed["dps"], "5")

	// measured data points are not exposed
	assert.Assert(t, findAction(actions, "setTemperature") == nil)
	assert.Assert(t, findAction(actions, "setFault") == nil)
	assert.Assert(t, findAction(actions, "setFilterLife") == nil)
}

// a device is only advertised with the data points it actually reports
func TestActionsForDeviceObservedOnly(t *testing.T) {
	// the plug reports DPS 1 but none of 2, 4, 5, 10 or 16
	observed := []string{"1", "18", "19", "41"}
	actions := actionsForDevice("tuya", "plug", DeviceConfig{}, observed)

	assert.Assert(t, findAction(actions, "turnOn") != nil)
	assert.Assert(t, findAction(actions, "setTargetHumidity") == nil)
	assert.Assert(t, findAction(actions, "setWorkMode") == nil)
	assert.Assert(t, findAction(actions, "setAnion") == nil)
	// an empty observed list means "nothing seen yet", not "everything"
	assert.Equal(t, len(actionsForDevice("tuya", "plug", DeviceConfig{}, []string{})), 0)
}

// an explicit name override wins over the canonical mapping
func TestCanonicalActionsOverrideByConfig(t *testing.T) {
	cfg := DeviceConfig{ActionNames: map[string]string{"1": "setSwitch"}}
	actions := actionsForDevice("tuya", "plug", cfg, nil)
	assert.Assert(t, findAction(actions, "setSwitch") != nil)
	assert.Assert(t, findAction(actions, "turnOn") == nil)
}

func TestActionsForDeviceConfigOverride(t *testing.T) {
	cfg := DeviceConfig{ActionNames: map[string]string{
		"5":  "setMode",   // rename
		"4":  "",          // hide
		"23": "setFilter", // explicitly expose a measured point
	}}
	actions := actionsForDevice("tuya", "dev", cfg, nil)
	assert.Assert(t, findAction(actions, "setMode") != nil)
	assert.Assert(t, findAction(actions, "setWorkMode") == nil)
	assert.Assert(t, findAction(actions, "setFanSpeed") == nil)
	assert.Assert(t, findAction(actions, "setFilter") != nil)
}

func TestActionNameForCustomDPS(t *testing.T) {
	cfg := DeviceConfig{DPSNames: map[string]string{"105": "power", "106": "timer"}}
	// unknown ids are only exposed when the table or the config marks them writable
	assert.Equal(t, actionNameFor(cfg, "105", "power"), "")
	assert.Equal(t, actionNameFor(cfg, "106", "timer"), "")
	cfg.WritableDPS = []string{"106"}
	assert.Equal(t, actionNameFor(cfg, "106", "timer"), "setTimer")
	// an explicit override exposes them regardless
	cfg.ActionNames = map[string]string{"105": "setPower"}
	assert.Equal(t, actionNameFor(cfg, "105", "power"), "setPower")
}

// a data point whose name was blanked out in the config gets no action, and must not panic
func TestActionNameForEmptyDPSName(t *testing.T) {
	cfg := DeviceConfig{DPSNames: map[string]string{"5": ""}}
	assert.Equal(t, actionNameFor(cfg, "5", ""), "")
	assert.Equal(t, len(actionsForDevice("tuya", "dev", cfg, []string{"5"})), 0)
}

// the friendly name and the observed-only actions reach the device registry
func TestRegisterDeviceActions(t *testing.T) {
	ctx, _, _ := helper.SetupEngineWithCommonOperators(t, nil)
	opSensor := device.GetGlobalSensors()

	op := &OpTuya{name: "tuya", config: TuyaConfig{SensorCategory: "tuyatest"}}
	op.config.Devices = map[string]DeviceConfig{"outlet": {Name: "WZ Stecker"}}
	w := newWatcher(op, "outlet", &Device{}, DeviceConfig{},
		base.NewBaseContextWithReason(logrus.New(), "test"))
	w.lastDPS = map[string]interface{}{"1": true}
	op.watchers = map[string]*deviceWatcher{"outlet": w}

	op.registerDeviceActions(ctx, "outlet")

	res := opSensor.GetActions(ctx, base.MakeEmptyOutput(), device.DeviceArgs{DeviceCategory: "tuyatest", DeviceName: "outlet"})
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	actions := map[string]device.ActionInfo{}
	assert.NilError(t, res.ParseJSON(&actions))
	_, hasOn := actions["turnOn"]
	assert.Assert(t, hasOn)
	_, hasHumidity := actions["setTargetHumidity"]
	assert.Assert(t, !hasHumidity)

	alias := opSensor.GetSensorAlias(ctx, base.MakeEmptyOutput(), device.SensorArgs{SensorCategory: "tuyatest", SensorName: "outlet"})
	assert.Equal(t, alias.GetString(), "WZ Stecker")
}
