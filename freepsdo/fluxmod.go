package freepsdo

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/utils"
)

type FluxMod struct {
	ff  *freepsflux.FreepsFlux
	ffc *freepsflux.FreepsFluxConfig
}

var _ Mod = &FluxMod{}

func NewFluxMod(cr *utils.ConfigReader) *FluxMod {
	ffc := &freepsflux.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	ff, _ := freepsflux.NewFreepsFlux(ffc, nil)
	return &FluxMod{ffc: ffc, ff: ff}
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

func (m *FluxMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	var err error
	if fn == "pushfields" {
		fields := map[string]interface{}{}

		var args JsonArgs
		err = json.Unmarshal(jsonStr, &args)
		if len(args.Measurement) == 0 {
			jrw.WriteError(http.StatusBadGateway, "Name of measurement is empty")
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
				jrw.WriteError(http.StatusBadRequest, "Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fwt.FieldValue, fwt.FieldType, err)
				return
			}
			fields[k] = value
		}

		err = m.ff.PushFields(args.Measurement, args.Tags, fields)
		if err == nil {
			jrw.WriteSuccessf("Pushed to influx: %v %v %v", args.Measurement, args.Tags, fields)
		} else {
			jrw.WriteError(http.StatusInternalServerError, "%v", err)
		}
	}
	return
}

func (m *FluxMod) GetFunctions() []string {
	return []string{"pushfields"}
}

func (m *FluxMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *FluxMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}
