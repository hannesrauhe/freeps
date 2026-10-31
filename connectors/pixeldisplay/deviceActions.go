package pixeldisplay

import (
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
)

// deviceCategory is the sensor category shared by all pixeldisplay instances.
const deviceCategory = "pixeldisplay"

// deviceName is the device name of this instance: the instance part of the operator name, "Default" for the plain one.
func (op *OpPixelDisplay) deviceName() string {
	name := op.name
	if len(name) >= len(deviceCategory) && strings.EqualFold(name[:len(deviceCategory)], deviceCategory) {
		name = name[len(deviceCategory):]
	}
	name = strings.ReplaceAll(strings.TrimPrefix(name, "."), ".", "_")
	if name == "" {
		return "Default"
	}
	return name
}

// actionsForDisplay exposes the display's own operator functions as readable actions. Pure, so it needs no device.
func actionsForDisplay(operatorName string) []*device.Action {
	return []*device.Action{
		{Name: "turnOn", Operator: operatorName, Function: "TurnOn"},
		{Name: "turnOff", Operator: operatorName, Function: "TurnOff"},
		{Name: "setBrightness", Operator: operatorName, Function: "SetBrightness",
			Param: &device.ActionParam{Name: "brightness", Type: "int", Scale: 2.55, Doc: "brightness in percent"}},
		{Name: "setColor", Operator: operatorName, Function: "SetColor",
			Param: &device.ActionParam{Name: "color", Type: "string", Doc: "hex color, e.g. #ff0000"}},
		{Name: "drawText", Operator: operatorName, Function: "DrawText",
			Param: &device.ActionParam{Name: "text", Type: "string"}},
	}
}

// setSensorProperty updates one property of this display, used to keep the sensor in sync with the display.
func (op *OpPixelDisplay) setSensorProperty(ctx *base.Context, property string, value interface{}) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		return
	}
	if err := opSensor.SetSensorPropertyInternal(ctx, deviceCategory, op.deviceName(), property, value); err != nil {
		ctx.GetLogger().Errorf("Failed to update pixeldisplay property %s: %v", property, err)
	}
}

// registerDevice publishes the display as a device with its actions.
func (op *OpPixelDisplay) registerDevice(ctx *base.Context) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		ctx.GetLogger().Debugf("sensor integration not available, cannot register pixeldisplay device")
		return
	}
	if err := opSensor.SetSensorPropertiesInternal(ctx, deviceCategory, op.deviceName(),
		map[string]interface{}{"name": op.deviceName()}); err != nil {
		ctx.GetLogger().Errorf("Failed to register pixeldisplay device: %v", err)
	}
	if err := opSensor.SetDeviceActionsInternal(deviceCategory, op.deviceName(), actionsForDisplay(op.name)); err != nil {
		ctx.GetLogger().Errorf("Failed to register pixeldisplay actions: %v", err)
	}
}
