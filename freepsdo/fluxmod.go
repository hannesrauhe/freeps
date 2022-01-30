package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/utils"
)

type FluxMod struct {
	ffc *freepsflux.FreepsFluxConfig
}

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

type JsonArgs struct {
	Measurement string
	Tags        map[string]string
	Fields      map[string]interface{}
}

func (m *FluxMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	ff, err := freepsflux.NewFreepsFlux(m.ffc, nil)
	if err != nil {
		log.Fatalf("Error while creating FreepsFlux: %v\n", err)
	}
	if fn == "pushfields" {
		var args JsonArgs
		err = json.Unmarshal(jsonStr, &args)
		err = ff.PushFields(args.Measurement, args.Tags, args.Fields)
	}
	return
}
