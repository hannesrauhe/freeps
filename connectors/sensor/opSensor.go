package sensor

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/jeremywohl/flatten"
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

// getSensorID calculates the sensor ID from the category and name
func (op *OpSensor) getSensorID(category string, name string) (string, error) {
	if strings.Contains(category, ".") || strings.Contains(name, ".") {
		return "", fmt.Errorf("SensorName and SensorCategory cannot contain a dot")
	}
	if category == "*" || name == "*" {
		return "", fmt.Errorf("SensorName and SensorCategory cannot be \"*\"")
	}
	if category == "" || name == "" {
		return "", fmt.Errorf("SensorName and SensorCategory cannot be empty")
	}
	return fmt.Sprintf("%s.%s", category, name), nil
}

func (o *OpSensor) getSensorCategories() ([]string, error) {
	ns := o.getSensorNamespace()
	v := ns.GetValue("_categories")
	if v.IsError() {
		return []string{}, v.GetError()
	}
	categories, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return []string{}, fmt.Errorf("category index is in an invalid format")
	}
	return categories.GetOriginalKeys(), nil
}

func (op *OpSensor) SensornameSuggestions() []string {
	return []string{"test"}
}

func (op *OpSensor) SensorcategorySuggestions() []string {
	cat, _ := op.getSensorCategories()
	return cat
}

// GetSensorCategories returns all sensor categories
func (op *OpSensor) GetSensorCategories(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	categories, err := op.getSensorCategories()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	return base.MakeObjectOutput(categories)
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
	sensorID, err := op.getSensorID(args.SensorCategory, args.SensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}

	ns := op.getSensorNamespace()
	newSensor := false
	updatedProperties := make([]string, 0)
	ent := ns.UpdateTransaction(sensorID, func(v freepsstore.StoreEntry) *base.OperatorIO {
		if v.IsError() {
			newSensor = true
			updatedProperties = fa.GetOriginalKeys()
			return base.MakeObjectOutput(fa)
		}
		existingProperties, ok := v.GetData().Output.(base.FunctionArguments)
		if !ok {
			return base.MakeOutputError(http.StatusInternalServerError, "existing properties for \"%s\" are in an invalid format", sensorID)
		}
		for _, k := range fa.GetOriginalKeys() {
			newPropertyValues := fa.GetValues(k)
			slices.Sort(newPropertyValues)

			if existingProperties.Has(k) {
				existingPropertyValues := existingProperties.GetValues(k)
				slices.Sort(existingPropertyValues)
				if 0 == slices.Compare(existingPropertyValues, newPropertyValues) {
					continue
				}
			}

			updatedProperties = append(updatedProperties, k)
			existingProperties.Set(k, newPropertyValues)
		}
		return base.MakeObjectOutput(existingProperties)
	}, ctx)
	if ent.IsError() {
		return ent.GetData()
	}

	if newSensor || len(updatedProperties) > 0 {
		// TODO(HR): async?
		op.executeTrigger(ctx, args.SensorCategory, sensorID, updatedProperties)
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

func (op *OpSensor) SetSensorPropertyInternal(ctx *base.Context, sensorCategory string, sensorName string, properties interface{}) error {
	m1, err := utils.ObjectToMap(properties)
	if err != nil {
		return err
	}
	m2, err := flatten.Flatten(m1, "", flatten.DotStyle)
	if err != nil {
		return err
	}
	m3 := make(map[string]string)
	for k, v := range m2 {
		m3[k] = fmt.Sprintf("%v", v)
	}
	fa := base.NewFunctionArguments(m3)
	io := op.SetSensorProperty(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCategory}, fa)
	if io.IsError() {
		return io.GetError()
	}
	return nil
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
	sensorID, err := op.getSensorID(args.SensorCategory, args.SensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	ns := op.getSensorNamespace()
	v := ns.GetValue(sensorID)
	completeSensorEntry := v.GetData()
	if args.PropertyName == nil {
		return completeSensorEntry
	}
	if completeSensorEntry.IsError() {
		return completeSensorEntry
	}

	properties, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "existing properties for \"%s\" are in an invalid format", sensorID)
	}

	if properties.Has(*args.PropertyName) {
		return base.MakePlainOutput(properties.Get(*args.PropertyName))
	}
	return base.MakeOutputError(404, "property \"%s\" not found", *args.PropertyName)
}
