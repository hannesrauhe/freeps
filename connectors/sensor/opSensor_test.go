package sensor

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func initSensorOp(t *testing.T) (*OpSensor, *base.Context) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	ge := freepsflow.NewFlowEngine(ctx, cr, func() {})
	tempOp := &OpSensor{CR: cr, GE: ge}
	base.MakeFreepsOperators(tempOp, cr, ctx)

	return GetGlobalSensor(), ctx
}
func TestSensorPropertySetting(t *testing.T) {
	op, ctx := initSensorOp(t)

	sensorCat := "test"
	sensorName := "test_sensor"
	sensorProperty := "test_property"
	res := op.SetSensorProperty(ctx, base.MakeEmptyOutput(), SensorArgs{Name: sensorName, Category: sensorCat}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, res.IsError())

	// set property of new sensor
	res = op.SetSensorProperty(ctx, base.MakeEmptyOutput(), SensorArgs{Name: sensorName, Category: sensorCat}, base.NewSingleFunctionArgument(sensorProperty, "test_value"))
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{Name: sensorName, Category: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "\"test_value\"")

	// update/overwrite property of existing sensor
	res = op.SetSensorProperty(ctx, base.MakeEmptyOutput(), SensorArgs{Name: sensorName, Category: sensorCat}, base.NewSingleFunctionArgument(sensorProperty, "test_value_new"))
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{Name: sensorName, Category: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "\"test_value_new\"")

	// update/overwrite property of existing sensor from input
	res = op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(12), SetSensorPropertyArgs{Name: sensorName, Category: sensorCat, PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{Name: sensorName, Category: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "12")
}
