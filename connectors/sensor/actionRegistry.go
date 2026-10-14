package sensor

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

// Actions live in memory only: operators rebuild them from the live device list on every poll.
// Like sensors, a removed device keeps its actions until restart.
var (
	actionLock    sync.RWMutex
	actionStorage = map[string]map[string]*Action{}
)

// SetDeviceActionsInternal replaces all actions of a device, which keeps registration idempotent across polls.
func (op *OpSensor) SetDeviceActionsInternal(sensorCategory string, sensorName string, actions []*Action) error {
	id, err := op.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return err
	}
	byName := map[string]*Action{}
	for _, action := range actions {
		if action.Name == "" {
			return fmt.Errorf("action for %s has an empty name", id)
		}
		if action.Operator == "" || action.Function == "" {
			return fmt.Errorf("action \"%s\" of %s has no target operator or function", action.Name, id)
		}
		byName[utils.StringToLower(action.Name)] = action
	}

	actionLock.Lock()
	defer actionLock.Unlock()
	actionStorage[id] = byName
	return nil
}

// DeleteActionsInternal removes all actions of a device.
func (op *OpSensor) DeleteActionsInternal(sensorCategory string, sensorName string) error {
	id, err := op.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return err
	}
	actionLock.Lock()
	defer actionLock.Unlock()
	delete(actionStorage, id)
	return nil
}

// getDeviceActions returns the actions of one device, sorted by name.
func getDeviceActions(sensorID string) []*Action {
	actionLock.RLock()
	defer actionLock.RUnlock()
	actions := make([]*Action, 0, len(actionStorage[sensorID]))
	for _, action := range actionStorage[sensorID] {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].Name < actions[j].Name })
	return actions
}

// getDeviceAction returns a single action of a device, or nil.
func getDeviceAction(sensorID string, actionName string) *Action {
	actionLock.RLock()
	defer actionLock.RUnlock()
	return actionStorage[sensorID][utils.StringToLower(actionName)]
}

// getActionDeviceIDs returns the IDs of all devices that have actions.
func getActionDeviceIDs() []string {
	actionLock.RLock()
	defer actionLock.RUnlock()
	ids := make([]string, 0, len(actionStorage))
	for id := range actionStorage {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// categoryOfSensorID splits a sensor ID back into its category (the part before the first dot).
func categoryOfSensorID(sensorID string) string {
	category, _, found := strings.Cut(sensorID, ".")
	if !found {
		return sensorID
	}
	return category
}

// executeAction runs an action after translating its readable parameters to the raw operator arguments.
func (op *OpSensor) executeAction(ctx *base.Context, input *base.OperatorIO, sensorID string, action *Action, callerArgs base.FunctionArguments) *base.OperatorIO {
	args, err := action.buildArgs(callerArgs)
	if err != nil {
		return base.MakeOutputError(400, "%v", err)
	}
	if !op.GE.HasOperator(action.Operator) {
		return base.MakeInternalServerErrorOutput(fmt.Errorf("action \"%s\" refers to unknown operator \"%s\"", action.Name, action.Operator))
	}
	return op.GE.ExecuteOperatorByName(ctx, action.Operator, action.Function, args, input)
}
