package sensor

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/influx"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

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

func (o *OpSensor) setSensorPropertyNoTrigger(ctx *base.Context, input *base.OperatorIO, sensorCategory string, sensorName string, sensorProperty string) (*base.OperatorIO, bool, bool, bool) {
	sensorID, err := o.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error()), false, false, false
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
			return input
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

	// update the sensor if the property is new
	if newProperty {
		sensorEnt := ns.UpdateTransaction(sensorID, func(v freepsstore.StoreEntry) *base.OperatorIO {
			sensorInformation := Sensor{}
			if v.IsError() {
				newSensor = true
				sensorInformation.Properties = []string{sensorProperty}
			} else {
				ok := false
				sensorInformation, ok = v.GetData().Output.(Sensor)
				if !ok {
					return base.MakeInternalServerErrorOutput(fmt.Errorf("existing properties for \"%s\" are in an invalid format", sensorID))
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
	}

	// update the category index if the sensor is new
	if newSensor {
		catEnt := ns.UpdateTransaction("_categories", func(v freepsstore.StoreEntry) *base.OperatorIO {
			categories, ok := v.GetData().Output.(base.FunctionArguments)
			if !ok {
				return base.MakeInternalServerErrorOutput(fmt.Errorf("category index is in an invalid format"))
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

func (o *OpSensor) setSensorProperties(ctx *base.Context, sensorCategory string, sensorName string, properties map[string]interface{}) *base.OperatorIO {
	updatedProperties := map[string]interface{}{}
	for k, v := range properties {
		out, _, _, updated := o.setSensorPropertyNoTrigger(ctx, base.MakeOutputGuessType(v), sensorCategory, sensorName, k)
		if out.IsError() {
			return out
		}
		if updated {
			updatedProperties[k] = v
		}
	}
	if len(updatedProperties) > 0 {

		o.recordUpdatesAndTrigger(ctx, sensorCategory, sensorName, updatedProperties)
	}
	return base.MakeEmptyOutput()
}

func (o *OpSensor) getSensorPropertyByID(sensorID string, sensorProperty string) *base.OperatorIO {
	ns := o.getSensorNamespace()
	return ns.GetValue(sensorID + "." + sensorProperty).GetData()
}
func (o *OpSensor) getSensorProperty(sensorCategory string, sensorName string, sensorProperty string) *base.OperatorIO {
	sensorID, err := o.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
	}
	return o.getSensorPropertyByID(sensorID, sensorProperty)
}

func (o *OpSensor) getSensorAliasByID(sensorID string) *base.OperatorIO {
	ns := o.getSensorNamespace()
	for _, key := range o.config.AliasKeys {
		v := ns.GetValue(sensorID + "." + key)
		if !v.IsError() {
			return v.GetData()
		}
	}
	// check if that sensor even exists (we do this at the end because we assume that an alias exists so we save a lookup in most cases)
	v := ns.GetValue(sensorID)
	if v.IsError() {
		return v.GetData()
	}
	return base.MakePlainOutput(sensorID)
}

func (o *OpSensor) getSensorAlias(sensorCategory string, sensorName string) *base.OperatorIO {
	sensorID, err := o.getSensorID(sensorCategory, sensorName)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err.Error())
	}
	return o.getSensorAliasByID(sensorID)
}

func (o *OpSensor) recordUpdatesAndTrigger(ctx *base.Context, sensorCategory string, sensorName string, changedProperties map[string]interface{}) {
	o.executeTriggers(ctx, sensorCategory, sensorName, changedProperties)

	if o.config.InfluxInstancePerCategory == nil {
		return
	}
	//TODO(HR): ignore the name for now
	_, ok := o.config.InfluxInstancePerCategory[sensorCategory]
	if !ok {
		return
	}
	ii := influx.GetGlobalInfluxInstance()
	if ii == nil {
		return
	}

	measurement := sensorCategory + "." + sensorName
	out := ii.PushFieldsInternal(measurement, map[string]string{}, changedProperties, ctx)
	if out.IsError() {
		alertDuration := time.Minute // TODO(HR): configure?
		o.GE.SetSystemAlert(ctx, "sensor_write_error", "sensor", 3, out.GetError(), &alertDuration)
	}

}
