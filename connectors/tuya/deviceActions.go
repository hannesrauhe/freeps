//go:build !notuya

package tuya

import (
	"sort"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/device"
)

// writableDPS are the data point ids known to be settable, everything else stays read-only.
// Devices with unusual ids can list their settable points in the WritableDPS config.
var writableDPS = map[string]bool{
	"1":   true, // switch
	"2":   true, // targetHumidity
	"4":   true, // fanSpeed
	"5":   true, // workMode
	"10":  true, // anion
	"16":  true, // childLock
	"17":  true, // countdown
	"24":  true, // tempUnit
	"101": true, // switch
}

// actionsForDevice turns every writable data point into a set<Name> action, e.g. setWorkMode.
// When observed is non-nil, data points the device never reported are skipped, so a device is
// not advertised with actions it does not have. Pure, so it can be tested without a device.
func actionsForDevice(operatorName string, name string, cfg DeviceConfig, observed []string) []*device.Action {
	seen := make(map[string]bool, len(observed))
	for _, id := range observed {
		seen[id] = true
	}

	var actions []*device.Action
	// sorted iteration keeps the first of two equally named points (DPS 1 and 101 are both "switch")
	dpsNames := namesFor(cfg)
	dpsIDs := make([]string, 0, len(dpsNames))
	for dps := range dpsNames {
		dpsIDs = append(dpsIDs, dps)
	}
	sort.Strings(dpsIDs)

	for _, dps := range dpsIDs {
		if observed != nil && !seen[dps] {
			continue
		}
		actionName := actionNameFor(cfg, dps, dpsNames[dps])
		if actionName == "" {
			continue
		}
		_, overridden := cfg.ActionNames[dps]
		if !overridden {
			if canonical := canonicalActions(operatorName, name, dps, actionName); canonical != nil {
				actions = append(actions, canonical...)
				continue
			}
		}
		if device.HasActionName(actions, actionName) {
			continue
		}
		actions = append(actions, &device.Action{
			Name:     actionName,
			Operator: operatorName, Function: "setdps",
			Fixed: map[string]string{"device": name, "dps": dps},
			Param: &device.ActionParam{
				Name: "value", Type: "string",
				Doc: "value to set on " + dpsNames[dps],
			},
		})
	}
	return actions
}

// canonicalActions maps the derived names of universal capabilities to the canonical action
// names, so clients can control any device the same way. It returns nil for everything else.
func canonicalActions(operatorName string, name string, dps string, actionName string) []*device.Action {
	switch {
	case actionName == "setSwitch" && dps == "1":
		// the main switch has its own function with a bool argument, which takes the fixed state
		return []*device.Action{
			{Name: device.ActionTurnOn, Operator: operatorName, Function: "setswitch",
				Fixed: map[string]string{"device": name, "on": "true"}},
			{Name: device.ActionTurnOff, Operator: operatorName, Function: "setswitch",
				Fixed: map[string]string{"device": name, "on": "false"}},
		}
	case actionName == device.ActionSetTargetHumidity:
		return []*device.Action{{
			Name: device.ActionSetTargetHumidity, Operator: operatorName, Function: "setdps",
			Fixed: map[string]string{"device": name, "dps": dps},
			Param: &device.ActionParam{
				Name: "humidity", Raw: "value", Type: "int",
				Doc: "target humidity in percent",
			},
		}}
	}
	return nil
}

// actionNameFor returns the action name of a data point, or "" if it should not be exposed.
// The ActionNames config wins (empty = hide); writable points get a "set" prefix, so the
// property targetHumidity becomes the action setTargetHumidity.
func actionNameFor(cfg DeviceConfig, dps string, dpsName string) string {
	if configured, ok := cfg.ActionNames[dps]; ok {
		return configured
	}
	if !isWritable(cfg, dps) || dpsName == "" {
		// an empty configured name means the user does not want a property, and no action either
		return ""
	}
	return "set" + strings.ToUpper(dpsName[0:1]) + dpsName[1:]
}

// isWritable reports whether a data point can be set, from the built-in table or the device config.
func isWritable(cfg DeviceConfig, dps string) bool {
	if writableDPS[dps] {
		return true
	}
	for _, id := range cfg.WritableDPS {
		if id == dps {
			return true
		}
	}
	return false
}

// registerActions publishes the actions of all configured devices.
func (o *OpTuya) registerActions(ctx *base.Context) {
	for _, name := range o.deviceNames() {
		o.registerDeviceActions(ctx, name)
	}
}

// registerDeviceActions publishes one device's friendly name and its actions, limited to the
// data points its watcher has seen so far (all of them before the first report).
func (o *OpTuya) registerDeviceActions(ctx *base.Context, name string) {
	opSensor := device.GetGlobalSensors()
	if opSensor == nil {
		ctx.GetLogger().Debugf("sensor integration not available, cannot register tuya actions")
		return
	}
	cfg := o.config.Devices[name]
	if cfg.Name != "" {
		props := map[string]interface{}{"name": cfg.Name}
		if err := opSensor.SetSensorPropertiesInternal(ctx, o.config.SensorCategory, name, props); err != nil {
			ctx.GetLogger().Errorf("Failed to set name for tuya device %v: %v", name, err)
		}
	}
	var observed []string
	o.mu.Lock()
	w := o.watchers[name]
	o.mu.Unlock()
	if w != nil {
		observed = w.observedDPS()
	}
	actions := actionsForDevice(o.name, name, cfg, observed)
	if err := opSensor.SetDeviceActionsInternal(o.config.SensorCategory, name, actions); err != nil {
		ctx.GetLogger().Errorf("Failed to register actions for tuya device %v: %v", name, err)
	}
}
