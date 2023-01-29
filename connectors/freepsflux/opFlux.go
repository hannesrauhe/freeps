package freepsflux

import (
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
	log "github.com/sirupsen/logrus"
)

type OpFlux struct {
	ff  *FreepsFlux
	ffc *FreepsFluxConfig
}

var _ freepsgraph.FreepsOperator = &OpFlux{}

func NewFluxMod(cr *utils.ConfigReader) *OpFlux {
	ffc := &DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	ff, _ := NewFreepsFlux(ffc, nil)
	return &OpFlux{ffc: ffc, ff: ff}
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

// GetName returns the name of the operator
func (o *OpFlux) GetName() string {
	return "flux"
}

func (o *OpFlux) Execute(ctx *utils.Context, fn string, vars map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var err error
	switch fn {
	case "pushfields":
		{
			fields := map[string]interface{}{}

			var args JsonArgs
			input.ParseJSON(&args)
			if len(args.Measurement) == 0 {
				return freepsgraph.MakeOutputError(http.StatusBadRequest, "Name of measurement is empty")
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
					return freepsgraph.MakeOutputError(http.StatusBadRequest, "Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fwt.FieldValue, fwt.FieldType, err)
				}
				fields[k] = value
			}

			err = o.ff.PushFields(args.Measurement, args.Tags, fields)
			if err == nil {
				return freepsgraph.MakePlainOutput("Pushed to influx: %v %v %v", args.Measurement, args.Tags, fields)
			} else {
				return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err)
			}
		}
	case "pushsinglefield":
		{
			m := vars["measurement"]
			fields := map[string]interface{}{vars["field"]: input.Output}
			delete(vars, "measurement")
			delete(vars, "field")

			err = o.ff.PushFields(m, vars, fields)
			if err == nil {
				return freepsgraph.MakePlainOutput("Pushed to influx: %v %v %v", m, vars, fields)
			} else {
				return freepsgraph.MakeOutputError(http.StatusInternalServerError, "%v", err)
			}
		}
	case "pushfreepsdevicelist":
		{
			return o.pushFreepsDeviceList(input)
		}
	case "pushfreepsmetrics":
		{
			return o.pushFreepsMetrics(input)
		}
	case "pushfreepsdata":
		{
			return o.pushFreepsData(input)
		}
	}
	return freepsgraph.MakeOutputError(http.StatusBadRequest, "Unknown function: %v", fn)
}

func (o *OpFlux) pushFreepsDeviceList(input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var devicelist freepslib.AvmDeviceList
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsDeviceList(&devicelist)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return freepsgraph.MakePlainOutput("%v", lp)
}

func (o *OpFlux) pushFreepsData(input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var devicelist freepslib.AvmDataResponse
	err := input.ParseJSON(&devicelist)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsNetDeviceList(&devicelist)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error when pushing netdevice list: %v", err)
	}
	return freepsgraph.MakePlainOutput("%v", lp)
}

func (o *OpFlux) pushFreepsMetrics(input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var metrics freepslib.FritzBoxMetrics
	err := input.ParseJSON(&metrics)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Error when parsing JSON: %v", err)
	}
	err, lp := o.ff.PushFreepsMetrics(&metrics)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "Error when pushing device list: %v", err)
	}
	return freepsgraph.MakePlainOutput("%v", lp)
}

func (o *OpFlux) GetFunctions() []string {
	return []string{"pushfields", "pushsinglefield", "pushfreepsdevicelist", "pushfreepsmetrics", "pushfreepsdata"}
}

func (o *OpFlux) GetPossibleArgs(fn string) []string {
	return []string{}
}

func (o *OpFlux) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpFlux) Shutdown(ctx *utils.Context) {
}
