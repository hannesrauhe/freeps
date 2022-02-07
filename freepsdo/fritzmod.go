package freepsdo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

type FritzMod struct {
	fl  *freepslib.Freeps
	fc  *freepslib.FBconfig
	ffc *freepsflux.FreepsFluxConfig
}

var _ Mod = &FritzMod{}

func NewFritzMod(cr *utils.ConfigReader) *FritzMod {
	conf := freepslib.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepslib", &conf)
	if err != nil {
		log.Fatal(err)
	}
	ffc := freepsflux.DefaultConfig
	err = cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	f, _ := freepslib.NewFreepsLib(&conf)
	return &FritzMod{fl: f, fc: &conf, ffc: &ffc}
}

func (m *FritzMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {
	var err error
	var vars map[string]string
	json.Unmarshal(jsonStr, &vars)

	if fn == "freepsflux" {
		ff, err2 := freepsflux.NewFreepsFlux(m.ffc, m.fl)
		if err2 != nil {
			log.Fatalf("Error while executing function: %v\n", err2)
		}
		ff.Verbose = m.fl.Verbose
		err = ff.Push()
		return
	} else if fn == "getdevicelistinfos" {
		devl, err := m.fl.GetDeviceList()
		if err != nil {
			fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError when getting device list: %v", vars, string(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jsonbytes, err := json.MarshalIndent(devl, "", "  ")
		if err != nil {
			fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError when creating JSON reponse: %v", vars, string(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(jsonbytes)
		return
	}

	dev := vars["device"]
	if fn == "wakeup" {
		log.Printf("Waking Up %v", dev)
		err = m.fl.WakeUpDevice(dev)
	} else {
		err = m.fl.HomeAutoSwitch(fn, dev, vars)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError: %v", vars, string(err.Error()))
		return
	}
	fmt.Fprintf(w, "Fritz: %v, %v, %v", fn, dev, vars)
}
