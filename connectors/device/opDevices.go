package device

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// GetDeviceCategories returns all categories that have devices
func (op *OpDevice) GetDeviceCategories(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	return op.GetSensorCategories(ctx, input)
}

// GetDevicesByAliasArgs returns a alias->DeviceInfo map filtered by properties
type GetDevicesByAliasArgs struct {
	PropertyName   []string `doc:"the properties to return, at least one of PropertyName or ActionName"`
	ActionName     []string `doc:"the actions to return, a device without any of them is left out"`
	DeviceCategory *string  `doc:"only list the devices of this category"`
}

// DeviceInfo is one device with its live property values and its executable actions.
type DeviceInfo struct {
	ID         string
	Properties map[string]interface{} `json:",omitempty"`
	Actions    []string               `json:",omitempty"`
}

// GetDevicesByAlias returns the requested properties and actions of the devices that have them, keyed by alias.
func (op *OpDevice) GetDevicesByAlias(ctx *base.Context, input *base.OperatorIO, args GetDevicesByAliasArgs) *base.OperatorIO {
	if len(args.PropertyName) == 0 && len(args.ActionName) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No PropertyName or ActionName provided")
	}

	categories, err := op.getCategoryIndex()
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}

	ret := map[string]DeviceInfo{}
	for deviceCategory, deviceNames := range categories.GetOriginalCaseMap() {
		if args.DeviceCategory != nil && !utils.StringEqualsIgnoreCase(deviceCategory, *args.DeviceCategory) {
			continue
		}
		for _, deviceName := range deviceNames {
			sensorID, err := op.getSensorID(deviceCategory, deviceName)
			if err != nil {
				return base.MakeInternalServerErrorOutput(err)
			}
			device := DeviceInfo{ID: sensorID, Properties: map[string]interface{}{}}
			for _, propertyName := range args.PropertyName {
				property := op.getSensorPropertyByID(sensorID, propertyName)
				if property.IsError() {
					continue
				}
				device.Properties[propertyName] = property.Output
			}
			for _, action := range selectedActions(sensorID, args.ActionName) {
				device.Actions = append(device.Actions, action.Name)
			}
			if len(args.ActionName) > 0 && len(device.Actions) == 0 {
				continue
			}
			if len(device.Properties) == 0 && len(device.Actions) == 0 {
				continue
			}

			alias := op.getSensorAliasByID(sensorID).GetString()
			if _, ok := ret[alias]; ok {
				alias = fmt.Sprintf("%s (%s)", alias, sensorID)
			}
			ret[alias] = device
		}
	}
	return base.MakeObjectOutput(ret)
}

// GetDevicesPerPropertyArgs limits the device universe to one category.
type GetDevicesPerPropertyArgs struct {
	DeviceCategory *string `doc:"only list the devices of this category"`
}

// GetDevicesPerProperty returns a map where the keys are the properties and the values are the devices that have this property.
func (op *OpDevice) GetDevicesPerProperty(ctx *base.Context, input *base.OperatorIO, args GetDevicesPerPropertyArgs) *base.OperatorIO {
	return op.GetSensorsPerProperty(ctx, input, GetSensorsPerPropertyArgs{SensorCategory: args.DeviceCategory})
}

// GetDevicesPerActionArgs limits the action universe to one category.
type GetDevicesPerActionArgs struct {
	DeviceCategory *string `doc:"only list the devices of this category"`
}

// GetDevicesPerAction returns a map where the keys are the actions and the values are the devices that have this action.
func (op *OpDevice) GetDevicesPerAction(ctx *base.Context, input *base.OperatorIO, args GetDevicesPerActionArgs) *base.OperatorIO {
	allActions := make(map[string][]string)
	for _, sensorID := range getActionDeviceIDs() {
		if args.DeviceCategory != nil && !utils.StringEqualsIgnoreCase(categoryOfSensorID(sensorID), *args.DeviceCategory) {
			continue
		}
		for _, action := range getDeviceActions(sensorID) {
			allActions[action.Name] = append(allActions[action.Name], sensorID)
		}
	}
	return base.MakeObjectOutput(allActions)
}

// selectedActions returns the actions of a device that the caller asked for, all of them if no names are given.
func selectedActions(sensorID string, wanted []string) []*Action {
	actions := getDeviceActions(sensorID)
	if len(wanted) == 0 {
		return actions
	}
	ret := []*Action{}
	for _, action := range actions {
		for _, name := range wanted {
			if utils.StringEqualsIgnoreCase(name, action.Name) {
				ret = append(ret, action)
				break
			}
		}
	}
	return ret
}
