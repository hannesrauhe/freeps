package sensor

import (
	"fmt"
	"net/http"
	"strings"

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
	if strings.ToLower(name) != "sensor" {
		return nil, fmt.Errorf("config section name must be 'sensor', multiple instances are not supported")
	}
	opc := config.(*SensorConfig)

	globalSensor = &OpSensor{CR: op.CR, GE: op.GE, config: opc}
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_sensors")
	if err != nil {
		return nil, err
	}
	ent := ns.SetValue("_categories", base.MakeObjectOutput(base.MakeEmptyFunctionArguments()), ctx)
	if ent.IsError() {
		return nil, ent.GetError()
	}
	return globalSensor, nil
}

// getSensorNanmespace returns the namespaces of the sensors
func (op *OpSensor) getSensorNamespace() freepsstore.StoreNamespace {
	return freepsstore.GetGlobalStore().GetNamespaceNoError("_sensors")
}

func (op *OpSensor) SensornameSuggestions() []string {
	return []string{"test"}
}

func (op *OpSensor) SensorcategorySuggestions() []string {
	return []string{"test"}
}

// GetSensorCategories returns all sensor categories
func (op *OpSensor) GetSensorCategories(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	ns := op.getSensorNamespace()
	v := ns.GetValue("_categories")
	if v.IsError() {
		return v.GetData()
	}
	categories, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "category index is in an invalid format")
	}
	return base.MakeObjectOutput(categories.GetOriginalKeys())
}

type GetSensorNamesArgs struct {
	SensorCategory string
}

// GetSensorNames returns all sensor names for a category
func (op *OpSensor) GetSensorNames(ctx *base.Context, input *base.OperatorIO, args GetSensorNamesArgs) *base.OperatorIO {
	ns := op.getSensorNamespace()
	v := ns.GetValue(args.SensorCategory)
	if v.IsError() {
		return v.GetData()
	}
	names, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "sensor index is in an invalid format")
	}
	return base.MakeObjectOutput(names.GetValues(args.SensorCategory))
}

type SensorArgs struct {
	SensorName     string
	SensorCategory string
}

// SetSensorProperty writes a one or more properties of a sensor
func (op *OpSensor) SetSensorProperty(ctx *base.Context, input *base.OperatorIO, args SensorArgs, fa base.FunctionArguments) *base.OperatorIO {
	if fa.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no properties to set")
	}
	//SensorName and SensorCategory are cannot be empty or contain a dot
	if strings.Contains(args.SensorName, ".") || strings.Contains(args.SensorCategory, ".") {
		return base.MakeOutputError(http.StatusBadRequest, "SensorName and SensorCategory cannot contain a dot")
	}

	ns := op.getSensorNamespace()
	key := fmt.Sprintf("%s.%s", args.SensorCategory, args.SensorName)
	newSensor := false
	ent := ns.UpdateTransaction(key, func(v freepsstore.StoreEntry) *base.OperatorIO {
		if v.IsError() {
			newSensor = true
			return base.MakeObjectOutput(fa)
		}
		existingProperties, ok := v.GetData().Output.(base.FunctionArguments)
		if !ok {
			return base.MakeOutputError(http.StatusInternalServerError, "existing properties for \"%s\" are in an invalid format", key)
		}
		// TODO: check before overwriting if value has changed
		for _, k := range fa.GetOriginalKeys() {
			existingProperties.Set(k, fa.GetValues(k))
		}
		return base.MakeObjectOutput(existingProperties)
	}, ctx)
	if ent.IsError() {
		return ent.GetData()
	}
	if !newSensor {
		return base.MakeEmptyOutput()
	}
	catEnt := ns.UpdateTransaction("_categories", func(v freepsstore.StoreEntry) *base.OperatorIO {
		if v.IsError() {
			return v.GetData()
		}
		categories, ok := v.GetData().Output.(base.FunctionArguments)
		if !ok {
			return base.MakeErrorOutputFromError(fmt.Errorf("category index is in an invalid format"))
		}
		if !categories.ContainsValue(args.SensorCategory, args.SensorName) {
			categories.Append(args.SensorCategory, args.SensorName)
		}
		return base.MakeObjectOutput(categories)
	}, ctx)
	if catEnt.IsError() {
		return catEnt.GetData()
	}
	return base.MakeEmptyOutput()

}

type SetSensorPropertyArgs struct {
	SensorName     string
	SensorCategory string
	PropertyName   string
}

// SetSingleSensorProperty writes a one or more properties of a sensor
func (op *OpSensor) SetSingleSensorProperty(ctx *base.Context, input *base.OperatorIO, args SetSensorPropertyArgs) *base.OperatorIO {
	fa := base.NewSingleFunctionArgument(args.PropertyName, input.GetString())
	return op.SetSensorProperty(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: args.SensorName, SensorCategory: args.SensorCategory}, fa)
}

type GetSensorArgs struct {
	SensorName     string
	SensorCategory string
	PropertyName   *string
}

// GetSensorProperty returns all properties of a sensor or the one specified by PropertyName
func (op *OpSensor) GetSensorProperty(ctx *base.Context, input *base.OperatorIO, args GetSensorArgs) *base.OperatorIO {
	//SensorName and SensorCategory are cannot be empty or contain a dot
	if strings.Contains(args.SensorName, ".") || strings.Contains(args.SensorCategory, ".") {
		return base.MakeOutputError(http.StatusBadRequest, "SensorName and SensorCategory cannot contain a dot")
	}
	ns := op.getSensorNamespace()
	key := fmt.Sprintf("%s.%s", args.SensorCategory, args.SensorName)
	v := ns.GetValue(key)
	completeSensorEntry := v.GetData()
	if args.PropertyName == nil {
		return completeSensorEntry
	}
	if completeSensorEntry.IsError() {
		return completeSensorEntry
	}

	properties, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "existing properties for \"%s\" are in an invalid format", key)
	}

	if properties.Has(*args.PropertyName) {
		return base.MakePlainOutput(properties.Get(*args.PropertyName))
	}
	return base.MakeOutputError(404, "property \"%s\" not found", *args.PropertyName)
}
