package sensor

import (
	"fmt"
	"net/http"
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

type Sensor struct {
	Properties []string
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

func (o *OpSensor) getCategoryIndex() (base.FunctionArguments, error) {
	ns := o.getSensorNamespace()
	v := ns.GetValue("_categories")
	if v.IsError() {
		return nil, v.GetError()
	}
	categories, ok := v.GetData().Output.(base.FunctionArguments)
	if !ok {
		return nil, fmt.Errorf("category index is in an invalid format")
	}
	return categories, nil
}

func (o *OpSensor) getSensorCategories() ([]string, error) {
	categories, err := o.getCategoryIndex()
	if err != nil {
		return []string{}, err
	}
	return categories.GetOriginalKeys(), nil
}

func (o *OpSensor) getPropertyIndex(sensorID string) (Sensor, error) {
	ns := o.getSensorNamespace()
	v := ns.GetValue(sensorID)
	if v.IsError() {
		return Sensor{}, v.GetError()
	}
	sensorInformation, ok := v.GetData().Output.(Sensor)
	if !ok {
		return Sensor{}, fmt.Errorf("existing properties for \"%s\" are in an invalid format", sensorID)
	}
	return sensorInformation, nil
}

func (o *OpSensor) setSensorProperty(ctx *base.Context, input *base.OperatorIO, sensorCategory string, sensorName string, sensorProperty string) (*base.OperatorIO, bool, bool, bool) {
	if input.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no properties to set"), false, false, false
	}
	sensorID, err := o.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error()), false, false, false
	}

	ns := o.getSensorNamespace()
	propertyKey := sensorID + "." + sensorProperty

	updatedProperty := false
	newProperty := false
	newSensor := false

	ent := ns.UpdateTransaction(propertyKey, func(oldProp freepsstore.StoreEntry) *base.OperatorIO {
		if oldProp.IsError() {
			updatedProperty = true
			newProperty = true
		}
		oldP := oldProp.GetData().GetString()
		if len(oldP) >= base.MAXSTRINGLENGTH {
			updatedProperty = true
		} else if oldP != input.GetString() {
			updatedProperty = true
		}
		if !updatedProperty {
			// tell the store that nothing has changed and to not touch the value
			out := base.MakeEmptyOutput()
			out.HTTPCode = http.StatusContinue
			return out
		}
		return input
	}, ctx)

	if ent.IsError() {
		return ent.GetData(), false, false, false
	}

	sensorEnt := ns.UpdateTransaction(sensorID, func(v freepsstore.StoreEntry) *base.OperatorIO {
		sensorInformation := Sensor{}
		if v.IsError() {
			newSensor = true
			sensorInformation.Properties = []string{sensorProperty}
		} else {
			ok := false
			sensorInformation, ok = v.GetData().Output.(Sensor)
			if !ok {
				return base.MakeErrorOutputFromError(fmt.Errorf("existing properties for \"%s\" are in an invalid format", sensorID))
			}
			if newProperty {
				sensorInformation.Properties = append(sensorInformation.Properties, sensorProperty)
			}
		}
		return base.MakeObjectOutput(sensorInformation)
	}, ctx)

	if sensorEnt.IsError() {
		return sensorEnt.GetData(), false, false, false
	}

	if newSensor {
		catEnt := ns.UpdateTransaction("_categories", func(v freepsstore.StoreEntry) *base.OperatorIO {
			categories, ok := v.GetData().Output.(base.FunctionArguments)
			if !ok {
				return base.MakeErrorOutputFromError(fmt.Errorf("category index is in an invalid format"))
			}
			if !categories.ContainsValue(sensorCategory, sensorName) {
				categories.Append(sensorCategory, sensorName)
			}
			return base.MakeObjectOutput(categories)
		}, ctx)

		if catEnt.IsError() {
			return catEnt.GetData(), false, false, false
		}
	}

	return base.MakeEmptyOutput(), newSensor, newProperty, updatedProperty
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
	cat, err := op.getCategoryIndex()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	names := cat.GetValues(args.SensorCategory)
	if len(names) == 0 {
		return base.MakeOutputError(http.StatusNotFound, "Category %s not found", args.SensorCategory)
	}
	return base.MakeObjectOutput(names)
}

type SensorArgs struct {
	SensorName     string
	SensorCategory string
}

// SetSensorProperties writes a one or more properties of a sensor
func (o *OpSensor) SetSensorProperties(ctx *base.Context, input *base.OperatorIO, args SensorArgs, fa base.FunctionArguments) *base.OperatorIO {
	if fa.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no properties to set")
	}

	updatedProperties := make([]string, 0)
	for k, v := range fa.GetOriginalCaseMapOnlyFirst() {
		out, _, _, updated := o.setSensorProperty(ctx, base.MakePlainOutput(v), args.SensorCategory, args.SensorName, k)
		if out.IsError() {
			return out
		}
		if updated {
			updatedProperties = append(updatedProperties, k)
		}
	}
	if len(updatedProperties) > 0 {
		sensorID, _ := o.getSensorID(args.SensorCategory, args.SensorName)
		o.executeTrigger(ctx, args.SensorCategory, sensorID, updatedProperties)
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
	io := op.SetSensorProperties(ctx, base.MakeEmptyOutput(), SensorArgs{SensorName: sensorName, SensorCategory: sensorCategory}, fa)
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
func (o *OpSensor) SetSingleSensorProperty(ctx *base.Context, input *base.OperatorIO, args SetSensorPropertyArgs) *base.OperatorIO {
	out, _, _, updated := o.setSensorProperty(ctx, input, args.SensorCategory, args.SensorName, args.PropertyName)

	if out.IsError() {
		return out
	}

	if updated {
		sensorID, _ := o.getSensorID(args.SensorCategory, args.SensorName)
		o.executeTrigger(ctx, args.SensorCategory, sensorID, []string{args.PropertyName})
	}

	return out
}

type GetSensorArgs struct {
	SensorName     string
	SensorCategory string
	PropertyName   *string
}

// GetSensorProperty returns all properties of a sensor or the one specified by PropertyName
func (o *OpSensor) GetSensorProperty(ctx *base.Context, input *base.OperatorIO, args GetSensorArgs) *base.OperatorIO {
	sensorID, err := o.getSensorID(args.SensorCategory, args.SensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	ns := o.getSensorNamespace()
	if args.PropertyName != nil {
		return ns.GetValue(sensorID + "." + *args.PropertyName).GetData()
	}

	v := ns.GetValue(sensorID)
	if v.IsError() {
		return v.GetData()
	}
	return v.GetData()
}

// GetSensorAlias returns the property "name" for the sensor or the id if this property does not exist
func (o *OpSensor) GetSensorAlias(ctx *base.Context, input *base.OperatorIO, args SensorArgs) *base.OperatorIO {
	sensorID, err := o.getSensorID(args.SensorCategory, args.SensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	ns := o.getSensorNamespace()
	v := ns.GetValue(sensorID + ".alias")
	if !v.IsError() {
		return v.GetData()
	}
	v = ns.GetValue(sensorID + ".name")
	if !v.IsError() {
		return v.GetData()
	}
	// check if that sensor even exists
	v = ns.GetValue(sensorID)
	if v.IsError() {
		return v.GetData()
	}
	return base.MakePlainOutput(sensorID)
}

// GetSensorProperties returns all properties of a sensor
func (o *OpSensor) GetSensorProperties(ctx *base.Context, input *base.OperatorIO, args SensorArgs) *base.OperatorIO {
	sensorID, err := o.getSensorID(args.SensorCategory, args.SensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	sensorInformation, err := o.getPropertyIndex(sensorID)
	if err != nil {
		return base.MakeOutputError(http.StatusNotFound, err.Error())
	}
	return base.MakeObjectOutput(sensorInformation.Properties)
}

// GetAllProperties returns all properties of the sensors in a category
func (o *OpSensor) GetAllProperties(ctx *base.Context, input *base.OperatorIO, args GetSensorNamesArgs) *base.OperatorIO {
	categories, err := o.getCategoryIndex()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	allProperties := make(map[string]string)
	for _, sensor := range categories.GetValues(args.SensorCategory) {
		sensorID, err := o.getSensorID(args.SensorCategory, sensor)
		if err != nil {
			return base.MakeErrorOutputFromError(err)
		}
		sensorInformation, err := o.getPropertyIndex(sensorID)
		if err != nil {
			return base.MakeErrorOutputFromError(err)
		}
		for _, property := range sensorInformation.Properties {
			allProperties[property] = sensorID
		}
	}

	return base.MakeObjectOutput(allProperties)
}
