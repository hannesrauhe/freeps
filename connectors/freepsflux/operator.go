//go:build !noinflux

package freepsflux

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
)

// OperatorFlux is that enabled InfluxDB Flux queries to be executed
type OperatorFlux struct {
	config *FreepsFluxConfig
	ff     *FreepsFlux
}

var _ base.FreepsOperatorWithConfig = &OperatorFlux{}

// GetDefaultConfig returns a copy of the default config
func (o *OperatorFlux) GetDefaultConfig() interface{} {
	return &FreepsFluxConfig{[]InfluxdbConfig{}, false, true, "_influx"}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (o *OperatorFlux) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	var err error
	newO := OperatorFlux{config: config.(*FreepsFluxConfig)}
	newO.ff, err = NewFreepsFlux(newO.config, nil)
	return &newO, err
}

func (o *OperatorFlux) PushFreepsDeviceList(ctx *base.Context, input *base.OperatorIO, args base.FunctionArguments) *base.OperatorIO {
	var devicelist freepslib.AvmDeviceList
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, _ = o.ff.PushFreepsDeviceList(&devicelist, args.GetLowerCaseMap())
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return base.MakeEmptyOutput()
}

func (o *OperatorFlux) PushFreepsData(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	var devicelist freepslib.AvmDataResponse
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, _ = o.ff.PushFreepsNetDeviceList(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing netdevice list: %v", err)
	}
	return base.MakeEmptyOutput()
}

func (o *OperatorFlux) PushFreepsMetrics(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	var metrics freepslib.FritzBoxMetrics
	err := input.ParseJSON(&metrics)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, _ = o.ff.PushFreepsMetrics(&metrics)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return base.MakeEmptyOutput()
}

type FieldWithType struct {
	FieldType  string
	FieldValue string
}
type JsonArgs struct {
	Measurement      string
	Tags             map[string]string
	Fields           map[string]interface{}
	FieldsWithType   map[string]FieldWithType
	DefaultFieldType string
}

func changeFieldType(fieldValue interface{}, fieldType string) (interface{}, error) {
	var value interface{}
	var err error
	fieldType = strings.ToLower(fieldType)
	switch fieldType {
	case "float", "float64":
		value, err = utils.ConvertToFloat(fieldValue)
	case "int", "int64":
		value, err = utils.ConvertToInt64(fieldValue)
	case "bool":
		value, err = utils.ConvertToBool(fieldValue)
	default:
		value = fieldValue
	}
	if err != nil {
		return value, fmt.Errorf("Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fieldValue, fieldType, err)
	}
	return value, nil
}

func (o *OperatorFlux) PushFields(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	fields := map[string]interface{}{}

	var args JsonArgs
	var err error
	input.ParseJSON(&args)
	if len(args.Measurement) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "Name of measurement is empty")
	}
	for k, v := range args.Fields {
		fields[k], err = changeFieldType(v, args.DefaultFieldType)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
		}
	}
	for k, v := range args.FieldsWithType {
		fields[k], err = changeFieldType(v.FieldValue, v.FieldType)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
		}
	}

	err = o.ff.PushFields(args.Measurement, args.Tags, fields, ctx)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
	return base.MakeEmptyOutput()
}

type PushArguments struct {
	Measurement string
	Field       string
	FieldType   *string
}

func (o *OperatorFlux) FieldTypeSuggestions() []string {
	return []string{"float", "float64", "int", "int64", "bool"}
}

func (o *OperatorFlux) PushSingleField(ctx *base.Context, input *base.OperatorIO, args PushArguments, tags map[string]string) *base.OperatorIO {
	fields := map[string]interface{}{}
	var err error
	if args.FieldType == nil {
		fields[args.Field] = input.Output
	} else {
		fields[args.Field], err = changeFieldType(input.Output, *args.FieldType)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}

	err = o.ff.PushFields(args.Measurement, tags, fields, ctx)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
	return base.MakeEmptyOutput()
}

type PushMeasurementArguments struct {
	Measurement string
}

func (o *OperatorFlux) PushMeasurement(ctx *base.Context, input *base.OperatorIO, args PushArguments, tags map[string]string) *base.OperatorIO {
	if input.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no input")
	}
	fields := map[string]interface{}{}
	err := input.ParseJSON(&fields)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Could not parse input: %v", err)
	}
	if len(fields) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "empty fields map")
	}

	err = o.ff.PushFields(args.Measurement, tags, fields, ctx)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
	return base.MakeEmptyOutput()
}
