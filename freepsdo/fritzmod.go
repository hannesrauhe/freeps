package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

type FritzMod struct {
	fl  *freepslib.Freeps
	ff  *freepsflux.FreepsFlux
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
	ff, _ := freepsflux.NewFreepsFlux(&ffc, f)
	return &FritzMod{fl: f, ff: ff, fc: &conf, ffc: &ffc}
}

func (m *FritzMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	var err error
	var vars map[string]string
	json.Unmarshal(jsonStr, &vars)

	if fn == "freepsflux" {
		err = m.ff.Push()
		if err != nil {
			jrw.WriteError(http.StatusInternalServerError, "Freepsflux error when pushing: %v", err.Error())
		}
		return
	} else if fn == "getdevicelistinfos" {
		devl, err := m.fl.GetDeviceList()
		if err != nil {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
			return
		}
		jrw.WriteSuccessMessage(devl)
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
		jrw.WriteError(http.StatusInternalServerError, err.Error())
		return
	}
	jrw.WriteSuccessf("%v, %v, %v", fn, dev, vars)
}
