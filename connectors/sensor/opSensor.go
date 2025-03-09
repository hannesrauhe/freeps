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

func (op *OpSensor) SensorNameSuggestions(otherArgs base.FunctionArguments) map[string]string {
	cm, err := op.getCategoryIndex()
	if err != nil {
		return map[string]string{}
	}
	cats := cm.GetOriginalKeys()
	if otherArgs.Has("SensorCategory") {
		cats = []string{otherArgs.Get("SensorCategory")}
	}
	ret := make(map[string]string)
	for _, cat := range cats {
		sensors := cm.GetValues(cat)
		for _, sensor := range sensors {
			alias := op.getSensorAlias(cat, sensor).GetString()
			_, already := ret[sensor]
			if !already {
				ret[alias] = sensor
			} else {
				ret[fmt.Sprintf("%s (%s)", alias, sensor)] = sensor
			}
		}
	}
	return ret
}

func (op *OpSensor) SensorCategorySuggestions() []string {
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

type GetSensorsPerCategoryArgs struct {
	SensorCategory *string
}

// GetSensorsPerCategory returns all sensor names for a category
func (op *OpSensor) GetSensorsPerCategory(ctx *base.Context, input *base.OperatorIO, args GetSensorsPerCategoryArgs) *base.OperatorIO {
	cat, err := op.getCategoryIndex()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	ret := make(map[string][]string)
	if args.SensorCategory != nil {
		if !cat.Has(*args.SensorCategory) {
			return base.MakeOutputError(http.StatusNotFound, "Category %s not found", *args.SensorCategory)
		}
		ret[*args.SensorCategory] = cat.GetValues(*args.SensorCategory)
	} else {
		ret = cat.GetOriginalCaseMap()
	}
	return base.MakeObjectOutput(ret)
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
		out, _, _, updated := o.setSensorProperty(ctx, base.MakeOutputGuessType(v), args.SensorCategory, args.SensorName, k)
		if out.IsError() {
			return out
		}
		if updated {
			updatedProperties = append(updatedProperties, k)
		}
	}
	if len(updatedProperties) > 0 {
		o.executeTrigger(ctx, args.SensorCategory, args.SensorName, updatedProperties)
	}
	return base.MakeEmptyOutput()
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
		o.executeTrigger(ctx, args.SensorCategory, args.SensorName, []string{args.PropertyName})
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
	return o.getSensorAlias(args.SensorCategory, args.SensorName)
}

// GetSensorPropertyKeys returns all properties of a sensor
func (o *OpSensor) GetSensorPropertyKeys(ctx *base.Context, input *base.OperatorIO, args SensorArgs) *base.OperatorIO {
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

type GetSensorsPerPropertyArgs struct {
	SensorCategory *string
}

// GetSensorsPerProperty returns a map where the keys are the properties and the values are the sensors that have this property
func (o *OpSensor) GetSensorsPerProperty(ctx *base.Context, input *base.OperatorIO, args GetSensorsPerPropertyArgs) *base.OperatorIO {
	categories, err := o.getCategoryIndex()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}
	categoriesList := categories.GetOriginalKeys()
	if args.SensorCategory != nil {
		categoriesList = []string{*args.SensorCategory}
	}
	allProperties := make(map[string][]string)
	for _, category := range categoriesList {
		for _, sensor := range categories.GetValues(category) {
			sensorID, err := o.getSensorID(category, sensor)
			if err != nil {
				return base.MakeErrorOutputFromError(err)
			}
			sensorInformation, err := o.getPropertyIndex(sensorID)
			if err != nil {
				return base.MakeErrorOutputFromError(err)
			}
			for _, property := range sensorInformation.Properties {
				if _, ok := allProperties[property]; !ok {
					allProperties[property] = []string{sensorID}
				} else {
					allProperties[property] = append(allProperties[property], sensorID)
				}
			}
		}
	}
	return base.MakeObjectOutput(allProperties)
}

type GetSensorPropertiesByAliasArgs struct {
	SensorPropertyName []string
	SensorCategory     *string
}

// GetSensorPropertiesByAlias returns all sensors that have the given property by the sensor alias
func (o *OpSensor) GetSensorPropertiesByAlias(ctx *base.Context, input *base.OperatorIO, args GetSensorPropertiesByAliasArgs) *base.OperatorIO {
	if len(args.SensorPropertyName) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No SensorPropertyName provided")
	}

	categories, err := o.getCategoryIndex()
	if err != nil {
		return base.MakeErrorOutputFromError(err)
	}

	if args.SensorCategory != nil {
		if !categories.Has(*args.SensorCategory) {
			return base.MakeOutputError(http.StatusNotFound, "Category %s not found", *args.SensorCategory)
		}
	}

	ret := make(map[string]map[string]interface{})
	for sensorCategory, sensorIDs := range categories.GetOriginalCaseMap() {
		if args.SensorCategory != nil && !utils.StringEqualsIgnoreCase(sensorCategory, *args.SensorCategory) {
			continue
		}

		for _, sensorName := range sensorIDs {
			sensorID, err := o.getSensorID(sensorCategory, sensorName)
			if err != nil {
				return base.MakeErrorOutputFromError(err)
			}
			thisSensorProperties := map[string]interface{}{}
			for _, sensorPropertyName := range args.SensorPropertyName {
				property := o.getSensorPropertyByID(sensorID, sensorPropertyName)
				if property.IsError() {
					continue
				}
				thisSensorProperties[sensorPropertyName] = property.Output
			}
			if len(thisSensorProperties) == 0 {
				continue
			}

			alias := o.getSensorAliasByID(sensorID).GetString()
			if _, ok := ret[alias]; ok {
				alias = fmt.Sprintf("%s (%s)", alias, sensorID)
			}
			ret[alias] = thisSensorProperties
		}
	}

	if len(ret) == 0 {
		if args.SensorCategory != nil {
			return base.MakeOutputError(http.StatusNotFound, "Property %s not found in category %s", args.SensorPropertyName, *args.SensorCategory)
		}
		return base.MakeOutputError(http.StatusNotFound, "Property %s not found", args.SensorPropertyName)
	}

	return base.MakeObjectOutput(ret)
}
