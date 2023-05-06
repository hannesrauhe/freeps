//go:build !noinflux

package freepsflux

import (
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freepslib"
)

// OperatorFlux is that enabled InfluxDB Flux queries to be executed
type OperatorFlux struct {
	config *FreepsFluxConfig
	ff     *FreepsFlux
}

// GetConfig returns the config struct o the operator that is filled with the values from the config file
func (o *OperatorFlux) GetConfig() interface{} {
	o.config = &DefaultConfig
	return o.config
}

// Init is called after the config is read and the operator is created
func (o *OperatorFlux) Init(ctx *base.Context) error {
	var err error
	o.ff, err = NewFreepsFlux(o.config, nil)
	return err
}

func (o *OperatorFlux) PushFreepsDeviceList(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	var devicelist freepslib.AvmDeviceList
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsDeviceList(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return base.MakePlainOutput("%v", lp)
}

func (o *OperatorFlux) PushFreepsData(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	var devicelist freepslib.AvmDataResponse
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsNetDeviceList(&devicelist)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing netdevice list: %v", err)
	}
	return base.MakePlainOutput("%v", lp)
}

func (o *OperatorFlux) PushFreepsMetrics(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	var metrics freepslib.FritzBoxMetrics
	err := input.ParseJSON(&metrics)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsMetrics(&metrics)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return base.MakePlainOutput("%v", lp)
}

type FieldWithType struct {
	FieldType  string
	FieldValue string
}
type JsonArgs struct {
	Measurement    string
	Tags           map[string]string
	Fields         map[string]interface{}
	FieldsWithType map[string]FieldWithType
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
		fields[k] = v
	}
	for k, fwt := range args.FieldsWithType {
		var value interface{}
		switch fwt.FieldType {
		case "float":
			value, err = strconv.ParseFloat(fwt.FieldValue, 64)
		case "int":
			value, err = strconv.Atoi(fwt.FieldValue)
		case "bool":
			value, err = strconv.ParseBool(fwt.FieldValue)
		default:
			value = fwt.FieldValue
		}
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fwt.FieldValue, fwt.FieldType, err)
		}
		fields[k] = value
	}

	err = o.ff.PushFields(args.Measurement, args.Tags, fields)
	if err == nil {
		return base.MakePlainOutput("Pushed to influx: %v %v %v", args.Measurement, args.Tags, fields)
	} else {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
}

type PushArguments struct {
	Measurement string
	Field       *string
}

func (o *OperatorFlux) PushSingleField(ctx *base.Context, input *base.OperatorIO, args PushArguments, tags map[string]string) *base.OperatorIO {
	if args.Field == nil {
		return base.MakeOutputError(http.StatusBadRequest, "Please specify a field name")
	}
	fields := map[string]interface{}{*args.Field: input.Output}

	err := o.ff.PushFields(args.Measurement, tags, fields)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
	return base.MakePlainOutput("Pushed to influx: %v", args, fields)
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

	err = o.ff.PushFields(args.Measurement, tags, fields)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}
	return base.MakePlainOutput("Pushed to influx: %v %v %v", args, fields)
}
