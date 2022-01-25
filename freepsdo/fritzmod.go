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
	fc  *freepslib.FBconfig
	ffc *freepsflux.FreepsFluxConfig
}

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
	return &FritzMod{&conf, &ffc}
}

func (m *FritzMod) Do(fn string, vars map[string][]string, w http.ResponseWriter) {
	f, err := freepslib.NewFreepsLib(m.fc)
	if err != nil {
		fmt.Fprintf(w, "FritzMod\nParameters: %v\nError on freepslib-init: %v", vars, string(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if fn == "freepsflux" {
		ff, err2 := freepsflux.NewFreepsFlux(m.ffc, f)
		if err2 != nil {
			log.Fatalf("Error while executing function: %v\n", err2)
		}
		ff.Verbose = f.Verbose
		err = ff.Push()
		return
	} else if fn == "getdevicelistinfos" {
		devl, err := f.GetDeviceList()
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

	dev := vars["device"][0]
	arg := make(map[string]string)
	for key, value := range vars {
		arg[key] = value[0]
	}
	if fn == "wakeup" {
		log.Printf("Waking Up %v", dev)
		err = f.WakeUpDevice(dev)
	} else {
		err = f.HomeAutoSwitch(fn, dev, arg)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "FritzHandler\nParameters: %v\nError: %v", vars, string(err.Error()))
		return
	}
	fmt.Fprintf(w, "Fritz: %v, %v, %v", fn, dev, arg)
}
