//go:build !noinflux

package influx

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// OperatorFlux is that enabled InfluxDB Flux queries to be executed
type OperatorFlux struct {
	config   *InfluxConfig
	writeApi api.WriteAPI
}

var _ base.FreepsOperatorWithConfig = &OperatorFlux{}

// GetDefaultConfig returns a copy of the default config
func (o *OperatorFlux) GetDefaultConfig() interface{} {
	return &InfluxConfig{Enabled: false}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (o *OperatorFlux) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	return &OperatorFlux{config: config.(*InfluxConfig)}, nil
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

	return o.PushFieldsInternal(args.Measurement, args.Tags, fields, ctx)
}

type PushArguments struct {
	Measurement string
	Field       string
	FieldType   *string
}

func (o *OperatorFlux) FieldTypeSuggestions() []string {
	return []string{"float", "float64", "int", "int64", "bool"}
}

func (o *OperatorFlux) PushSingleField(ctx *base.Context, input *base.OperatorIO, args PushArguments, tags base.FunctionArguments) *base.OperatorIO {
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

	return o.PushFieldsInternal(args.Measurement, tags.GetOriginalCaseMapOnlyFirst(), fields, ctx)
}

type PushMeasurementArguments struct {
	Measurement string
}

func (o *OperatorFlux) PushMeasurement(ctx *base.Context, input *base.OperatorIO, args PushMeasurementArguments, tags base.FunctionArguments) *base.OperatorIO {
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

	return o.PushFieldsInternal(args.Measurement, tags.GetOriginalCaseMapOnlyFirst(), fields, ctx)
}
