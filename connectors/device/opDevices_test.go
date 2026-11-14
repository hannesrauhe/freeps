package device_test

import (
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"gotest.tools/v3/assert"
)

func TestGetDeviceCategories(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "cat1", "dev", map[string]interface{}{"name": "d"}))

	res := op.GetDeviceCategories(ctx, base.MakeEmptyOutput())
	assert.Assert(t, !res.IsError(), "%v", res.GetError())
	categories := []string{}
	assert.NilError(t, res.ParseJSON(&categories))
	assert.Assert(t, len(categories) >= 1)
}

// The store and action registry are process wide, so this test uses its own categories and always filters by category.
func TestGetDevicesByAlias(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "ga", "lamp", map[string]interface{}{"name": "My Lamp", "brightness": 60}))
	assert.NilError(t, op.SetSensorPropertyInternal(ctx, "ga", "lamp", "_hidden", "raw blob"))
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "ga", "plug", map[string]interface{}{"name": "My Plug"}))
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "gb", "thermometer", map[string]interface{}{"brightness": 30}))

	assert.NilError(t, op.SetDeviceActionsInternal("ga", "lamp", []*device.Action{
		echoAction("turnOn", nil, nil), echoAction("setLevel", nil, &device.ActionParam{Name: "level", Type: "int"}),
	}))
	assert.NilError(t, op.SetDeviceActionsInternal("ga", "plug", []*device.Action{echoAction("turnOn", nil, nil)}))

	byAlias := func(category string, args device.GetDevicesByAliasArgs) map[string]device.DeviceInfo {
		args.DeviceCategory = &category
		res := op.GetDevicesByAlias(ctx, base.MakeEmptyOutput(), args)
		assert.Assert(t, !res.IsError(), "%v", res.GetError())
		out := map[string]device.DeviceInfo{}
		assert.NilError(t, res.ParseJSON(&out))
		return out
	}

	all := byAlias("ga", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}})
	assert.Equal(t, len(all), 2)

	lamp := all["My Lamp"]
	assert.Equal(t, lamp.ID, "ga.lamp")
	// values come back through JSON, so numbers arrive as float64
	assert.Equal(t, lamp.Properties["brightness"], 60.0)
	_, leaked := lamp.Properties["_hidden"]
	assert.Assert(t, !leaked, "unrequested property leaked: %v", lamp.Properties)
	assert.Equal(t, lamp.Actions[0], "setLevel")

	// the actuator is listed with its actions alone, which GetSensorPropertiesByAlias would drop
	assert.Equal(t, all["My Plug"].ID, "ga.plug")
	assert.Equal(t, len(all["My Plug"].Properties), 0)
	assert.Equal(t, len(all["My Plug"].Actions), 1)

	// ActionName narrows the returned actions and filters out devices without them
	turnOn := byAlias("ga", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}, ActionName: []string{"turnOn"}})
	assert.Equal(t, len(turnOn), 2)
	assert.Equal(t, len(turnOn["My Lamp"].Actions), 1)
	assert.Equal(t, turnOn["My Lamp"].Actions[0], "turnOn")

	setLevel := byAlias("ga", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}, ActionName: []string{"setLevel", "unknown"}})
	assert.Equal(t, len(setLevel), 1)
	assert.Equal(t, setLevel["My Lamp"].ID, "ga.lamp")

	none := byAlias("ga", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}, ActionName: []string{"doesNotExist"}})
	assert.Equal(t, len(none), 0)

	// ActionName alone is valid and lists the devices that have the action, with their actions only
	byAction := byAlias("ga", device.GetDevicesByAliasArgs{ActionName: []string{"turnOn"}})
	assert.Equal(t, len(byAction), 2)
	assert.Equal(t, len(byAction["My Plug"].Properties), 0)
	assert.Equal(t, len(byAction["My Plug"].Actions), 1)
	assert.Equal(t, byAction["My Plug"].Actions[0], "turnOn")

	// a sensor with the property but no actions is keyed by its id, as it has no name property
	gb := byAlias("gb", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}})
	assert.Equal(t, len(gb), 1)
	assert.Equal(t, gb["gb.thermometer"].ID, "gb.thermometer")
	assert.Equal(t, len(gb["gb.thermometer"].Actions), 0)

	empty := byAlias("noSuchCategory", device.GetDevicesByAliasArgs{PropertyName: []string{"brightness"}})
	assert.Equal(t, len(empty), 0)

	res := op.GetDevicesByAlias(ctx, base.MakeEmptyOutput(), device.GetDevicesByAliasArgs{})
	assert.Assert(t, res.IsError())
	assert.ErrorContains(t, res.GetError(), "No PropertyName or ActionName")
}

// Both universe functions are tested on the same devices, again filtered by own categories.
func TestGetDevicesPerPropertyAndAction(t *testing.T) {
	op, ctx := initActions(t)
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "uc", "lamp", map[string]interface{}{"name": "Lamp", "brightness": 10}))
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "uc", "plug", map[string]interface{}{"name": "Plug"}))

	assert.NilError(t, op.SetDeviceActionsInternal("uc", "lamp", []*device.Action{echoAction("turnOn", nil, nil)}))
	assert.NilError(t, op.SetDeviceActionsInternal("uc", "plug", []*device.Action{echoAction("turnOn", nil, nil), echoAction("toggle", nil, nil)}))

	perProperty := func(category string) map[string][]string {
		args := device.GetDevicesPerPropertyArgs{DeviceCategory: &category}
		res := op.GetDevicesPerProperty(ctx, base.MakeEmptyOutput(), args)
		assert.Assert(t, !res.IsError(), "%v", res.GetError())
		out := map[string][]string{}
		assert.NilError(t, res.ParseJSON(&out))
		return out
	}
	perAction := func(category string) map[string][]string {
		args := device.GetDevicesPerActionArgs{DeviceCategory: &category}
		res := op.GetDevicesPerAction(ctx, base.MakeEmptyOutput(), args)
		assert.Assert(t, !res.IsError(), "%v", res.GetError())
		out := map[string][]string{}
		assert.NilError(t, res.ParseJSON(&out))
		return out
	}

	props := perProperty("uc")
	assert.DeepEqual(t, props["name"], []string{"uc.lamp", "uc.plug"})
	assert.DeepEqual(t, props["brightness"], []string{"uc.lamp"})

	// a sensor in the category index without a property index must be skipped, not fail the request
	assert.NilError(t, op.SetSensorPropertiesInternal(ctx, "uc", "broken", map[string]interface{}{"name": "Broken"}))
	freepsstore.GetGlobalStore().GetNamespaceNoError("_devices").DeleteValue("uc.broken")
	assert.DeepEqual(t, perProperty("uc")["name"], []string{"uc.lamp", "uc.plug"})

	actions := perAction("uc")
	assert.DeepEqual(t, actions["turnOn"], []string{"uc.lamp", "uc.plug"})
	assert.DeepEqual(t, actions["toggle"], []string{"uc.plug"})

	assert.Equal(t, len(perProperty("noSuchCategory")), 0)
	assert.Equal(t, len(perAction("noSuchCategory")), 0)
}
