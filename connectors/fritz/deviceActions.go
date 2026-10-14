package fritz

import (
	"strconv"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freepslib"
)

// actionsForDevice derives the actions of a device from the switch commands freepslib knows
// about, keeping only what the device supports. It is pure, so it can be tested without a FritzBox.
func actionsForDevice(operatorName string, dev freepslib.AvmDevice) []*sensor.Action {
	if dev.AIN == "" {
		// buttons and other devices without an AIN cannot be switched
		return nil
	}
	capabilities := freepslib.GetSwitchCmdCapabilities()
	var actions []*sensor.Action
	for cmd, args := range freepslib.GetSwitchCmds() {
		capability, exposed := capabilities[cmd]
		if !exposed || !dev.HasCapability(capability) {
			continue
		}
		action := &sensor.Action{Name: cmd, Operator: operatorName, Function: cmd,
			Fixed: map[string]string{"device": dev.AIN}}
		for _, arg := range args {
			if arg == "device" {
				continue
			}
			// the raw commands treat every argument as optional
			action.Params = append(action.Params, sensor.ActionParam{Name: arg, Optional: true})
		}
		if cmd == "sethkrtsoll" && dev.HKR != nil {
			action.Params = hkrTemperatureParams(dev)
		}
		actions = append(actions, action)
	}
	return actions
}

// hkrTemperatureParams replaces the raw param argument with a readable °C temperature,
// using the device's own comfort and eco values for the preset labels.
func hkrTemperatureParams(dev freepslib.AvmDevice) []sensor.ActionParam {
	values := map[string]string{"off": "253"}
	if dev.HKR.Komfort > 0 && dev.HKR.Komfort < 253 {
		values["comfort"] = strconv.Itoa(dev.HKR.Komfort)
	}
	if dev.HKR.Absenk > 0 && dev.HKR.Absenk < 253 {
		values["eco"] = strconv.Itoa(dev.HKR.Absenk)
	}
	return []sensor.ActionParam{{
		Name: "temperature", Raw: "param", Type: "float", Scale: 2, Values: values,
		Doc: "target temperature in °C, or one of the preset labels",
	}}
}

// deviceToActions publishes the readable actions of one device.
func (o *OpFritz) deviceToActions(ctx *base.Context, dev freepslib.AvmDevice) {
	opSensor := sensor.GetGlobalSensors()
	if opSensor == nil {
		return
	}
	if err := opSensor.SetDeviceActionsInternal(o.getDeviceSensorCategory(), dev.DeviceID, actionsForDevice(o.name, dev)); err != nil {
		ctx.GetLogger().Errorf("Failed to register actions for device %v: %v", dev.Name, err)
	}
}
