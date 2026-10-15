package pixeldisplay

import (
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
)

// deviceName is the single device the display is registered as.
const deviceName = "display"

// deviceCategory is the sensor category the display is registered under, derived from the operator name.
func (op *OpPixelDisplay) deviceCategory() string {
	return strings.ReplaceAll(op.name, ".", "_")
}

// actionsForDisplay exposes the display's own operator functions as readable actions. Pure, so it needs no device.
func actionsForDisplay(operatorName string) []*device.Action {
	return []*device.Action{
		{Name: "turnOn", Operator: operatorName, Function: "TurnOn"},
		{Name: "turnOff", Operator: operatorName, Function: "TurnOff"},
		{Name: "setBrightness", Operator: operatorName, Function: "SetBrightness",
			Params: []device.ActionParam{{Name: "brightness", Type: "int", Doc: "0-255"}}},
		{Name: "setColor", Operator: operatorName, Function: "SetColor",
			Params: []device.ActionParam{{Name: "color", Type: "string", Doc: "hex color, e.g. #ff0000"}}},
		{Name: "drawText", Operator: operatorName, Function: "DrawText",
			Params: []device.ActionParam{{Name: "text", Type: "string"}}},
	}
}

// setSensorProperty updates one property of the display device, used to keep the sensor in sync with the display.
func (op *OpPixelDisplay) setSensorProperty(ctx *base.Context, property string, value interface{}) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		return
	}
	if err := opSensor.SetSensorPropertyInternal(ctx, op.deviceCategory(), deviceName, property, value); err != nil {
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
	category := op.deviceCategory()
	if err := opSensor.SetSensorPropertiesInternal(ctx, category, deviceName,
		map[string]interface{}{"name": "Pixel Display"}); err != nil {
		ctx.GetLogger().Errorf("Failed to register pixeldisplay device: %v", err)
	}
	if err := opSensor.SetDeviceActionsInternal(category, deviceName, actionsForDisplay(op.name)); err != nil {
		ctx.GetLogger().Errorf("Failed to register pixeldisplay actions: %v", err)
	}
}
