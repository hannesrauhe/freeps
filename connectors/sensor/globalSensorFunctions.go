package sensor

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
)

var globalSensor *OpSensor

// GetGlobalSensors returns the global sensor instance, that can be used by other operators to manage their sensors
func GetGlobalSensors() *OpSensor {
	return globalSensor
}

// SetSensorPropertyFromFlattenedObject sets the properties of a sensor by flattening a given object and setting the properties as key-value pairs where each value is converted to a string
func (op *OpSensor) SetSensorPropertyFromFlattenedObject(ctx *base.Context, sensorCategory string, sensorName string, properties interface{}) error {
	m1, err := utils.ObjectToMap(properties)
	if err != nil {
		return err
	}
	m2, err := flatten.Flatten(m1, "", flatten.DotStyle)
	if err != nil {
		return err
	}
	m3 := make(map[string]string)
	for k, v := range m2 {
		m3[k] = fmt.Sprintf("%v", v)
	}
	fa := base.NewFunctionArguments(m3)
	io := op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCategory}, fa)
	if io.IsError() {
		return io.GetError()
	}
	return nil
}

// GetSensorNamesInternal returns the names of all sensors of a given category
func (op *OpSensor) GetSensorNamesInternal(ctx *base.Context, sensorCategory string) ([]string, error) {
	cat, err := op.getCategoryIndex()
	if err != nil {
		return nil, err
	}
	return cat.GetValues(sensorCategory), nil
}

// GetSensorPropertyInternal returns the value of a sensor property
func (op *OpSensor) GetSensorPropertyInternal(ctx *base.Context, sensorCategory string, sensorName string, propertyName string) *base.OperatorIO {
	return op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCategory, PropertyName: &propertyName})
}

// SetSensorPropertyInternal sets the value of a sensor property
func (op *OpSensor) SetSensorPropertyInternal(ctx *base.Context, sensorCategory string, sensorName string, propertyName string, value interface{}) error {
	result, _, _, _ := op.setSensorProperty(ctx, base.MakeObjectOutput(value), sensorCategory, sensorName, propertyName)
	if result.IsError() {
		return result.GetError()
	}
	return nil
}
