package sensor

import (
	"path"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	freepsutils "github.com/hannesrauhe/freeps/connectors/utils"
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
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge},
		&freepsutils.OpUtils{},
		&OpSensor{CR: cr, GE: ge},
	}

	for _, op := range availableOperators {
		ge.AddOperators(base.MakeFreepsOperators(op, cr, ctx))
	}

	return GetGlobalSensors(), ctx
}

func TestSensorPropertySetting(t *testing.T) {
	op, ctx := initSensorOp(t)

	sensorCat := "test"
	sensorName := "test_sensor"
	sensorProperty := "test_property"
	res := op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCat}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, res.IsError())

	// set property of new sensor
	res = op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCat}, base.NewSingleFunctionArgument(sensorProperty, "test_value"))
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "test_value")

	// update/overwrite property of existing sensor
	res = op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCat}, base.NewSingleFunctionArgument(sensorProperty, "test_value_new"))
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "test_value_new")

	// update/overwrite property of existing sensor from input
	res = op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(12), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "12")

	// check property name is not case sensitive
	upProp := "TEST_PROPERTY"
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &upProp})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "12")
	res = op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(14), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: upProp})
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &upProp})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "14")
}

func TestSensorName(t *testing.T) {
	op, ctx := initSensorOp(t)

	sensorCat := "test"
	sensorName := "test_sensor"
	sensorProperty := "test_property"
	res := op.GetSensorAlias(ctx, base.MakeEmptyOutput(), SensorArgs{SensorCategory: sensorCat, SensorName: sensorName})
	assert.Assert(t, res.IsError())
	assert.Equal(t, res.HTTPCode, 404)

	res = op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(12), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())

	res = op.GetSensorAlias(ctx, base.MakeEmptyOutput(), SensorArgs{SensorCategory: sensorCat, SensorName: sensorName})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), sensorCat+"."+sensorName)

	res = op.SetSingleSensorProperty(ctx, base.MakePlainOutput("alias sensor name"), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: "name"})
	assert.Assert(t, !res.IsError())

	res = op.GetSensorAlias(ctx, base.MakeEmptyOutput(), SensorArgs{SensorCategory: sensorCat, SensorName: sensorName})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "alias sensor name")

	res = op.SetSingleSensorProperty(ctx, base.MakePlainOutput("sensor name"), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: "alias"})
	assert.Assert(t, !res.IsError())

	res = op.GetSensorAlias(ctx, base.MakeEmptyOutput(), SensorArgs{SensorCategory: sensorCat, SensorName: sensorName})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "sensor name")

	// allow empty properties
	sensorProperty = "empty_prop"
	res = op.SetSingleSensorProperty(ctx, base.MakeEmptyOutput(), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())
	res = op.GetSensorProperty(ctx, base.MakeEmptyOutput(), GetSensorArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: &sensorProperty})
	assert.Assert(t, !res.IsError())
	assert.Equal(t, res.GetString(), "")
}

func TestSensorCategory(t *testing.T) {
	op, ctx := initSensorOp(t)

	sensorProperty := "test_property"

	res := op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(12), SetSensorPropertyArgs{SensorName: "testsenscat1", SensorCategory: "cat1", PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())
	res = op.SetSingleSensorProperty(ctx, base.MakeIntegerOutput(12), SetSensorPropertyArgs{SensorName: "testsenscat2", SensorCategory: "cat2", PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())

	res = op.GetSensorCategories(ctx, base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError())
	assert.DeepEqual(t, res.GetObject(), []string{"cat1", "cat2"})

	cat1 := "cat1"
	res = op.GetSensorsPerCategory(ctx, base.MakeEmptyOutput(), GetSensorsPerCategoryArgs{SensorCategory: &cat1})
	assert.Assert(t, !res.IsError())
	assert.DeepEqual(t, res.GetObject(), map[string][]string{"cat1": {"testsenscat1"}})

	notExisting := "NOTEXISTING"
	res = op.GetSensorsPerCategory(ctx, base.MakeEmptyOutput(), GetSensorsPerCategoryArgs{SensorCategory: &notExisting})
	assert.Assert(t, res.IsError())
	assert.Equal(t, res.HTTPCode, 404)
}

func createTestFlow(keyToSet string) freepsflow.FlowDesc {
	gd := freepsflow.FlowDesc{Operations: []freepsflow.FlowOperationDesc{{Operator: "utils", Function: "echoArguments"}, {Operator: "store", Function: "set", InputFrom: "#0", Arguments: map[string]string{"namespace": "test", "key": keyToSet}}}}
	return gd
}

func TestTriggers(t *testing.T) {
	op, ctx := initSensorOp(t)
	ge := op.GE

	flow1 := "testflowCat1"
	flow2 := "testflowProp1"
	flow3 := "testflowID1"
	err := ge.AddFlow(ctx, flow1, createTestFlow(flow1), false)
	assert.NilError(t, err)
	err = ge.AddFlow(ctx, flow2, createTestFlow(flow2), false)
	assert.NilError(t, err)
	err = ge.AddFlow(ctx, flow3, createTestFlow(flow3), false)
	assert.NilError(t, err)

	testCat1 := "testcat1"
	testProp1 := "test_property1"
	testSensor1 := "test_sensor1"

	out := op.SetSensorTrigger(ctx, base.MakeEmptyOutput(), SetTriggerArgs{FlowID: flow1, SensorCategory: &testCat1})
	out = op.SetSensorTrigger(ctx, base.MakeEmptyOutput(), SetTriggerArgs{FlowID: flow2, ChangedProperty: &testProp1})
	out = op.SetSensorTrigger(ctx, base.MakeEmptyOutput(), SetTriggerArgs{FlowID: flow3, SensorName: &testSensor1})
	assert.Assert(t, !out.IsError())

	/* Test the triggers when sensor of the right category and property is activated*/
	op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: "test_sensor", SensorCategory: testCat1}, base.NewSingleFunctionArgument(testProp1, "test_value"))

	ns, err := freepsstore.GetGlobalStore().GetNamespace("test")
	assert.NilError(t, err)
	assert.Assert(t, ns.GetValue(flow1) != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue(flow2) != freepsstore.NotFoundEntry)

	i := ns.DeleteOlder(time.Duration(0))
	assert.Equal(t, i, 2)

	/* value has not been changed, don't do anything */
	op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: "test_sensor", SensorCategory: testCat1}, base.NewSingleFunctionArgument(testProp1, "test_value"))
	i = ns.DeleteOlder(time.Duration(0))
	assert.Assert(t, i == 0)

	/* other property changes, trigger flow 1 */
	op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: "test_sensor", SensorCategory: testCat1}, base.NewSingleFunctionArgument("other_prop", "test_value"))
	assert.Assert(t, ns.GetValue(flow1) != freepsstore.NotFoundEntry)
	i = ns.DeleteOlder(time.Duration(0))
	assert.Equal(t, i, 1)

	/* Test the triggers when sensor of the right category and property is activated via update*/
	op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: "test_sensor", SensorCategory: testCat1}, base.NewSingleFunctionArgument(testProp1, "test_value_new"))

	assert.NilError(t, err)
	assert.Assert(t, ns.GetValue(flow1) != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue(flow2) != freepsstore.NotFoundEntry)

	i = ns.DeleteOlder(time.Duration(0))
	assert.Equal(t, i, 2)

	/* Test the ID trigger */
	op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: "test_sensor1", SensorCategory: testCat1}, base.NewSingleFunctionArgument(testProp1, "test_value"))
	assert.Assert(t, ns.GetValue(flow1) != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue(flow2) != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue(flow3) != freepsstore.NotFoundEntry)
	i = ns.DeleteOlder(time.Duration(0))
	assert.Equal(t, i, 3)
}

func setSensorPropertyHelper(t *testing.T, op *OpSensor, ctx *base.Context, sensorCat string, sensorName string, sensorProperty string, value interface{}) {
	res := op.SetSingleSensorProperty(ctx, base.MakeOutputInferType(value), SetSensorPropertyArgs{SensorName: sensorName, SensorCategory: sensorCat, PropertyName: sensorProperty})
	assert.Assert(t, !res.IsError())
}

func TestGetSensorPropertiesByAlias(t *testing.T) {
	op, ctx := initSensorOp(t)

	setSensorPropertyHelper(t, op, ctx, "test", "sensor1", "intprop", 12)
	setSensorPropertyHelper(t, op, ctx, "test", "sensor1", "name", "alias sensor name")
	setSensorPropertyHelper(t, op, ctx, "test", "sensorWithoutIntProp", "name", "not relevant")
	setSensorPropertyHelper(t, op, ctx, "test", "sensorWithoutIntProp", "someprop", "foo")
	setSensorPropertyHelper(t, op, ctx, "test2", "sensor2", "name", "alias sensor name")
	setSensorPropertyHelper(t, op, ctx, "test2", "sensor2", "intprop", 22)
	setSensorPropertyHelper(t, op, ctx, "test2", "sensor3", "intprop", 22)
	setSensorPropertyHelper(t, op, ctx, "test2", "sensor3", "stringprop", "foo")
	setSensorPropertyHelper(t, op, ctx, "test2", "sensor4", "stringprop", "bar")

	res := op.GetSensorPropertiesByAlias(ctx, base.MakeEmptyOutput(), GetSensorPropertiesByAliasArgs{SensorPropertyName: []string{"intprop"}, SensorCategory: utils.StringPtr("test")})
	assert.Equal(t, res.GetString(), `{
  "alias sensor name": {
    "intprop": 12
  }
}`)

	res = op.GetSensorPropertiesByAlias(ctx, base.MakeEmptyOutput(), GetSensorPropertiesByAliasArgs{SensorCategory: utils.StringPtr("test")})
	assert.Assert(t, res.IsError())
	// works but depends on the hash map order
	//
	//		res = op.GetSensorPropertiesByAlias(ctx, base.MakeEmptyOutput(), GetSensorPropertiesByAliasArgs{SensorPropertyNames: []string{"intprop"}})
	//		assert.Equal(t, res.GetString(), `{
	//	  "alias sensor name": {
	//	    "intprop": 12
	//	  },
	//	  "alias sensor name (test2.sensor2)": {
	//	    "intprop": 22
	//	  },
	//	  "test2.sensor3": {
	//	    "intprop": 22
	//	  }
	//	}`)
	//
	//		res = op.GetSensorPropertiesByAlias(ctx, base.MakeEmptyOutput(), GetSensorPropertiesByAliasArgs{SensorPropertyNames: []string{"intprop", "stringprop"}})
	//		assert.Equal(t, res.GetString(), `{
	//	  "alias sensor name": {
	//	    "intprop": 22
	//	  },
	//	  "alias sensor name (test.sensor1)": {
	//	    "intprop": 12
	//	  },
	//	  "test2.sensor3": {
	//	    "intprop": 22,
	//	    "stringprop": "foo"
	//	  },
	//	  "test2.sensor4": {
	//	    "stringprop": "bar"
	//	  }
	//	}`)
}
