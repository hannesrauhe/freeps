package fritz

import (
	"strconv"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	"github.com/hannesrauhe/freepslib"
)

// canonicalForCapability maps an AVM capability to the canonical action names it provides,
// so a client can control any device without knowing the AVM command names.
var canonicalForCapability = map[string][]string{
	"switch":      {device.ActionTurnOn, device.ActionTurnOff, device.ActionToggle},
	"simpleonoff": {device.ActionTurnOn, device.ActionTurnOff},
	"level":       {device.ActionSetBrightness},
	"color":       {device.ActionSetColorTemperature},
	"hkr":         {device.ActionSetTargetTemperature},
}

// actionsForDevice derives the canonical actions of a device from the capabilities freepslib knows
// about, keeping only what the device supports. It is pure, so it can be tested without a FritzBox.
func actionsForDevice(operatorName string, dev freepslib.AvmDevice) []*device.Action {
	if dev.AIN == "" {
		// buttons and other devices without an AIN cannot be switched
		return nil
	}
	var actions []*device.Action
	for _, capability := range capabilityOrder {
		if !dev.HasCapability(capability) {
			continue
		}
		for _, name := range canonicalForCapability[capability] {
			// a device with both a switch and an on/off part gets one turnOn from the first
			if hasAction(actions, name) {
				continue
			}
			actions = append(actions, canonicalAction(name, capability, operatorName, dev))
		}
	}
	return actions
}

func hasAction(actions []*device.Action, name string) bool {
	for _, a := range actions {
		if a.Name == name {
			return true
		}
	}
	return false
}

// capabilityOrder keeps the actions of a device in a stable, useful order.
var capabilityOrder = []string{"simpleonoff", "switch", "level", "color", "hkr"}

// canonicalAction builds one canonical action for a capability, translating to the raw AVM command.
func canonicalAction(name string, capability string, operatorName string, dev freepslib.AvmDevice) *device.Action {
	action := &device.Action{Name: name, Operator: operatorName, Fixed: map[string]string{"device": dev.AIN}}
	switch name {
	case device.ActionTurnOn, device.ActionTurnOff:
		if capability == "simpleonoff" {
			// the on/off part takes the state as an argument, the plain switch has one command each
			action.Function = "setsimpleonoff"
			onoff := "0"
			if name == device.ActionTurnOn {
				onoff = "1"
			}
			action.Fixed["onoff"] = onoff
		} else {
			action.Function = "setswitch" + strings.ToLower(strings.TrimPrefix(name, "turn"))
		}
	case device.ActionToggle:
		action.Function = "setswitchtoggle"
	case device.ActionSetBrightness:
		action.Function = "setlevelpercentage"
		action.Param = &device.ActionParam{Name: "brightness", Raw: "level", Type: "int", Doc: "brightness in percent"}
	case device.ActionSetColorTemperature:
		action.Function = "setcolortemperature"
		action.Param = &device.ActionParam{Name: "temperature", Type: "int", Doc: "color temperature in Kelvin"}
	case device.ActionSetTargetTemperature:
		action.Function = "sethkrtsoll"
		action.Param = hkrTemperatureParam(dev)
	}
	return action
}

// hkrTemperatureParam builds the readable °C parameter, using the device's own comfort and eco values for the preset labels.
func hkrTemperatureParam(dev freepslib.AvmDevice) *device.ActionParam {
	values := map[string]string{"off": "253"}
	if dev.HKR.Komfort > 0 && dev.HKR.Komfort < 253 {
		values["comfort"] = strconv.Itoa(dev.HKR.Komfort)
	}
	if dev.HKR.Absenk > 0 && dev.HKR.Absenk < 253 {
		values["eco"] = strconv.Itoa(dev.HKR.Absenk)
	}
	return &device.ActionParam{
		Name: "temperature", Raw: "param", Type: "float", Scale: 2, Values: values,
		Doc: "target temperature in °C, or one of the preset labels",
	}
}

// deviceToActions publishes the readable actions of one device, devices without any are left out.
func (o *OpFritz) deviceToActions(ctx *base.Context, dev freepslib.AvmDevice) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		return
	}
	actions := actionsForDevice(o.name, dev)
	var err error
	if len(actions) == 0 {
		err = opSensor.DeleteActionsInternal(o.getDeviceSensorCategory(), dev.DeviceID)
	} else {
		err = opSensor.SetDeviceActionsInternal(o.getDeviceSensorCategory(), dev.DeviceID, actions)
	}
	if err != nil {
		ctx.GetLogger().Errorf("Failed to register actions for device %v: %v", dev.Name, err)
	}
}
