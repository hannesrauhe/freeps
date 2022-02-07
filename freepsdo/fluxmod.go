package freepsdo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/utils"
)

type FluxMod struct {
	ffc *freepsflux.FreepsFluxConfig
}

var _ Mod = &FluxMod{}

func NewFluxMod(cr *utils.ConfigReader) *FluxMod {
	ffc := freepsflux.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	return &FluxMod{&ffc}
}

func (m *FluxMod) Do(fn string, vars map[string][]string, w http.ResponseWriter) {
	ff, err := freepsflux.NewFreepsFlux(m.ffc, nil)
	if err != nil {
		log.Fatalf("Error while creating FreepsFlux: %v\n", err)
	}
	if fn == "pushfields" {
		tags := map[string]string{}
		json.Unmarshal([]byte(vars["tags"][0]), &tags)
		fields := map[string]interface{}{}
		json.Unmarshal([]byte(vars["fields"][0]), &fields)
		err = ff.PushFields(vars["measurement"][0], tags, fields)
	}
	return
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

func (m *FluxMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	ff, err := freepsflux.NewFreepsFlux(m.ffc, nil)
	if err != nil {
		log.Fatalf("Error while creating FreepsFlux: %v\n", err)
	}
	if fn == "pushfields" {
		fields := map[string]interface{}{}

		var args JsonArgs
		err = json.Unmarshal(jsonStr, &args)
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
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fwt.FieldValue, fwt.FieldType, err)
				return
			}
			fields[k] = value
		}

		err = ff.PushFields(args.Measurement, args.Tags, fields)
		if err == nil {
			fmt.Fprint(w, "Pushed to influx: ", args.Measurement, args.Tags, fields)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err, "\nTried to pushed to influx: ", args.Measurement, args.Tags, fields)
		}
	}
	return
}
