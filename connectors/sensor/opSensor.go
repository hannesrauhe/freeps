package sensor

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

// OpSensor is an operator to manage sensors of different types in your Smart Home, these sensors can be created by the user or by other operators. The operator provices a set of methods to interact with the sensors.
type OpSensor struct {
	CR     *utils.ConfigReader
	GE     *freepsflow.FlowEngine
	config *SensorConfig
}

var _ base.FreepsOperator = &OpSensor{}
var _ base.FreepsOperatorWithConfig = &OpSensor{}

func (op *OpSensor) GetDefaultConfig() interface{} {
	return &SensorConfig{Enabled: true}
}

func (op *OpSensor) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	if name != "sensor" {
		return nil, fmt.Errorf("config section name must be 'sensor', multiple instances are not supported")
	}
	opc := config.(*SensorConfig)

	globalSensor = &OpSensor{CR: op.CR, GE: op.GE, config: opc}
	return globalSensor, nil
}

// getSensorNanmespace returns the namespaces of the sensors
func (op *OpSensor) getSensorNamespace() freepsstore.StoreNamespace {
	return freepsstore.GetGlobalStore().GetNamespaceNoError("_sensors")
}

type SensorArgs struct {
	Name     string
	Category string
}

// SetSensorFromInput writes a complete sensor to the store
func (op *OpSensor) SetSensorFromInput(ctx *base.Context, input *base.OperatorIO, args SensorArgs) *base.OperatorIO {
	ns := op.getSensorNamespace()
	key := fmt.Sprintf("%s.%s", args.Category, args.Name)
	ent := ns.SetValue(key, input, ctx)
	if ent.IsError() {
		return ent.GetData()
	}
	return base.MakeEmptyOutput()
}

// SetSensorProperty writes a one or more properties of a sensor
func (op *OpSensor) SetSensorProperty(ctx *base.Context, input *base.OperatorIO, args SensorArgs, fa base.FunctionArguments) *base.OperatorIO {
	if fa.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no properties to set")
	}

	ns := op.getSensorNamespace()
	key := fmt.Sprintf("%s.%s", args.Category, args.Name)
	ent := ns.UpdateTransaction(key, func(v freepsstore.StoreEntry) *base.OperatorIO {
		newProperties := fa.GetOriginalCaseMapOnlyFirst()
		if v.IsError() {
			return base.MakeObjectOutput(newProperties)
		}
		existingProperties, err := v.GetData().GetMap()
		if err != nil {
			return base.MakeErrorOutputFromError(err)
		}
		// TODO: cannot just assign newProperties to existingProperties, because there might be case-sensitivity issues
		// TODO: check before overwriting if value has changed
		for k, v := range newProperties {
			existingProperties[k] = v
		}
		return base.MakeObjectOutput(existingProperties)
	}, ctx)
	if ent.IsError() {
		return ent.GetData()
	}
	return base.MakeEmptyOutput()
}

type GetSensorArgs struct {
	Name         string
	Category     string
	PropertyName *string
}

// GetSensorProperty returns all properties of a sensor or the one specified by PropertyName
func (op *OpSensor) GetSensorProperty(ctx *base.Context, input *base.OperatorIO, args GetSensorArgs) *base.OperatorIO {
	ns := op.getSensorNamespace()
	key := fmt.Sprintf("%s.%s", args.Category, args.Name)
	v := ns.GetValue(key)
	completeSensorEntry := v.GetData()
	if args.PropertyName == nil {
		return completeSensorEntry
	}
	if completeSensorEntry.IsError() {
		return completeSensorEntry
	}
	regM, err := completeSensorEntry.GetMap()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	m := utils.NewCIMap(regM)
	if m.Has(*args.PropertyName) {
		return base.MakeObjectOutput(m.Get(*args.PropertyName))
	}
	return base.MakeOutputError(404, "property \"%s\" not found", *args.PropertyName)
}
