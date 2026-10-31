package device

import (
	"net/http"
	"sort"

	"github.com/hannesrauhe/freeps/base"
)

// GetActions returns the actions that can be executed on a device, with their
// readable parameters. Operators like fritz or tuya register them automatically,
// so a user can control a device without knowing the operator functions behind it.
func (op *OpDevice) GetActions(ctx *base.Context, input *base.OperatorIO, args DeviceArgs) *base.OperatorIO {
	sensorID, err := op.getSensorID(args.DeviceCategory, args.DeviceName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	actions := getDeviceActions(sensorID)
	if len(actions) == 0 {
		return base.MakeOutputError(http.StatusNotFound, "No actions known for %s", sensorID)
	}
	ret := make(map[string]ActionInfo, len(actions))
	for _, action := range actions {
		ret[action.Name] = action.info()
	}
	return base.MakeObjectOutput(ret)
}

// GetActionsPerCategory returns all devices that have actions, by category. The
// alias of each device is included so that the device id can be mapped back to a
// friendly name without a second request.
func (op *OpDevice) GetActionsPerCategory(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	ret := map[string][]DeviceActions{}
	for _, sensorID := range getActionDeviceIDs() {
		actions := getDeviceActions(sensorID)
		names := make([]string, 0, len(actions))
		for _, action := range actions {
			names = append(names, action.Name)
		}
		ret[categoryOfSensorID(sensorID)] = append(ret[categoryOfSensorID(sensorID)], DeviceActions{
			ID:      sensorID,
			Alias:   op.getSensorAliasByID(sensorID).GetString(),
			Actions: names,
		})
	}
	return base.MakeObjectOutput(ret)
}

// DeviceArgs addresses one device, the same device a sensor of that name belongs to.
type DeviceArgs struct {
	DeviceName     string
	DeviceCategory string
}

// ActionArgs are the arguments of the functions that work on a single action.
type ActionArgs struct {
	DeviceName     string
	DeviceCategory string
	ActionName     string
	Value          string `doc:"the value for actions that take one, e.g. the brightness percent"`
}

// ValueSuggestions suggests the readable labels of the selected action's parameter.
func (args *ActionArgs) ValueSuggestions(otherArgs base.FunctionArguments) map[string]string {
	sensorID, err := sensorIDFromCategoryName(args.DeviceCategory, args.DeviceName)
	if err != nil {
		return map[string]string{}
	}
	action := getDeviceAction(sensorID, args.ActionName)
	if action == nil || action.Param == nil {
		return map[string]string{}
	}
	return action.Param.valueSuggestions()
}

// GetActionParams returns the value parameter of an action as a list of argument
// descriptions, with the readable name and labels the caller can send as value.
func (op *OpDevice) GetActionParams(ctx *base.Context, input *base.OperatorIO, args ActionArgs) *base.OperatorIO {
	sensorID, err := op.getSensorID(args.DeviceCategory, args.DeviceName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	action := getDeviceAction(sensorID, args.ActionName)
	if action == nil {
		return base.MakeOutputError(http.StatusNotFound, "No action \"%s\" for %s, available actions: %v", args.ActionName, sensorID, actionNames(sensorID))
	}
	return base.MakeObjectOutput(action.argumentDescriptions())
}

// ExecuteAction executes an action of a device and returns an error if it fails. Actions with a
// parameter take it as the value argument, e.g.
// /device/executeAction?deviceCategory=tuya&deviceName=dehumidifier&actionName=setTargetHumidity&value=60
func (op *OpDevice) ExecuteAction(ctx *base.Context, input *base.OperatorIO, args ActionArgs) *base.OperatorIO {
	sensorID, err := op.getSensorID(args.DeviceCategory, args.DeviceName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	action := getDeviceAction(sensorID, args.ActionName)
	if action == nil {
		return base.MakeOutputError(http.StatusNotFound, "No action \"%s\" for %s, available actions: %v", args.ActionName, sensorID, actionNames(sensorID))
	}
	if res := op.executeAction(ctx, input, action, args.Value); res.IsError() {
		return res
	}
	return base.MakeEmptyOutput()
}

// ActionNameSuggestions returns the actions of the given device, so that the
// flow editor can offer them.
func (op *OpDevice) ActionNameSuggestions(otherArgs base.FunctionArguments) map[string]string {
	if !otherArgs.Has("DeviceCategory") || !otherArgs.Has("DeviceName") {
		return map[string]string{}
	}
	sensorID, err := op.getSensorID(otherArgs.Get("DeviceCategory"), otherArgs.Get("DeviceName"))
	if err != nil {
		return map[string]string{}
	}
	ret := map[string]string{}
	for _, name := range actionNames(sensorID) {
		ret[name] = name
	}
	return ret
}

// DeviceNameSuggestions are the devices an action can be executed on, the same ones that have sensors.
func (op *OpDevice) DeviceNameSuggestions(otherArgs base.FunctionArguments) map[string]string {
	if otherArgs.Has("DeviceCategory") {
		translated := base.MakeEmptyFunctionArguments()
		translated.Set("SensorCategory", []string{otherArgs.Get("DeviceCategory")})
		return op.SensorNameSuggestions(translated)
	}
	return op.SensorNameSuggestions(otherArgs)
}

// DeviceCategorySuggestions are the categories of the devices that have sensors.
func (op *OpDevice) DeviceCategorySuggestions() []string {
	return op.SensorCategorySuggestions()
}

// actionNames returns the sorted names of all actions of a device.
func actionNames(sensorID string) []string {
	actions := getDeviceActions(sensorID)
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Name)
	}
	sort.Strings(names)
	return names
}
