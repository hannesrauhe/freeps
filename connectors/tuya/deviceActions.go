//go:build !notuya

package tuya

import (
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
)

// writableDPS are the data point ids known to be settable, everything else stays read-only.
var writableDPS = map[string]bool{
	"1":   true, // switch
	"2":   true, // setHumidity
	"4":   true, // fanSpeed
	"5":   true, // workMode
	"10":  true, // anion
	"16":  true, // childLock
	"17":  true, // countdown
	"24":  true, // tempUnit
	"101": true, // switch
}

// actionsForDevice turns every writable data point into a set<Name> action, e.g. setHumidity.
// It is pure so that it can be tested without a device.
func actionsForDevice(operatorName string, name string, cfg DeviceConfig) []*device.Action {
	var actions []*device.Action
	// sorted iteration keeps the first of two equally named points (DPS 1 and 101 are both "switch")
	dpsNames := namesFor(cfg)
	dpsIDs := make([]string, 0, len(dpsNames))
	for dps := range dpsNames {
		dpsIDs = append(dpsIDs, dps)
	}
	sort.Strings(dpsIDs)

	for _, dps := range dpsIDs {
		actionName := actionNameFor(cfg, dps, dpsNames[dps])
		if actionName == "" || hasAction(actions, actionName) {
			continue
		}
		actions = append(actions, &device.Action{
			Name:     actionName,
			Operator: operatorName, Function: "setdps",
			Fixed: map[string]string{"device": name, "dps": dps},
			Params: []device.ActionParam{{
				Name: "value", Type: "string",
				Doc: "value to set on " + dpsNames[dps],
			}},
		})
	}
	return actions
}

func hasAction(actions []*device.Action, name string) bool {
	for _, action := range actions {
		if action.Name == name {
			return true
		}
	}
	return false
}

// actionNameFor returns the action name of a data point, or "" if it should not be exposed.
// The ActionNames config wins (empty = hide); otherwise writable points named "set..." keep
// their name and the rest get a "set" prefix.
func actionNameFor(cfg DeviceConfig, dps string, dpsName string) string {
	if configured, ok := cfg.ActionNames[dps]; ok {
		return configured
	}
	if !writableDPS[dps] && !strings.HasPrefix(dpsName, "set") {
		return ""
	}
	if strings.HasPrefix(dpsName, "set") {
		return dpsName
	}
	return "set" + strings.ToUpper(dpsName[0:1]) + dpsName[1:]
}

// registerActions publishes the actions of all configured devices.
func (o *OpTuya) registerActions(ctx *base.Context) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		ctx.GetLogger().Debugf("sensor integration not available, cannot register tuya actions")
		return
	}
	for _, name := range o.deviceNames() {
		if err := opSensor.SetDeviceActionsInternal(o.config.SensorCategory, name, actionsForDevice(o.name, name, o.config.Devices[name])); err != nil {
			ctx.GetLogger().Errorf("Failed to register actions for tuya device %v: %v", name, err)
		}
	}
}
