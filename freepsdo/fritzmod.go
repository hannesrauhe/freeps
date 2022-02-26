package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"

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
		if err != nil {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
			return
		}
		jrw.WriteSuccessf("Woke up %s", dev)
	} else if fn[0:3] == "set" {
		err = m.fl.HomeAutoSwitch(fn, dev, vars)
		if err != nil {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
			return
		}
		jrw.WriteSuccessf("%v, %v, %v", fn, dev, vars)
	} else {
		var r []byte
		r, err = m.fl.HomeAutomation(fn, dev, vars)

		if err != nil {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
			return
		}
		jrw.WriteSuccessMessage(r)
	}
}

func (m *FritzMod) GetFunctions() []string {
	swc := m.fl.GetSuggestedSwitchCmds()
	switchcmds := make([]string, 0, len(swc))
	for k := range swc {
		switchcmds = append(switchcmds, k)
	}
	sort.Strings(switchcmds)
	return switchcmds
}

func (m *FritzMod) GetPossibleArgs(fn string) []string {
	swc := m.fl.GetSuggestedSwitchCmds()
	if f, ok := swc[fn]; ok {
		return f
	}
	return make([]string, 0)
}

func (m *FritzMod) GetArgSuggestions(fn string, arg string) map[string]string {
	switch arg {
	case "device":
		return m.GetDevices()
	case "onoff":
		return map[string]string{"On": "1", "Off": "0", "Toggle": "2"}
	case "param":
		return map[string]string{"Off": "253", "16": "32", "18": "36", "20": "40", "22": "44", "24": "48"}
	case "temperature": // fn=="setcolortemperature"
		return map[string]string{"2700K": "2700", "3500K": "3500", "4250K": "4250", "5000K": "5000", "6500K": "6500"}
	case "duration":
		return map[string]string{"0": "0", "0.1s": "1", "1s": "10"}
	}
	return map[string]string{}
}

func (m *FritzMod) GetDevices() map[string]string {
	retMap := map[string]string{}
	devl, err := m.fl.GetDeviceList()
	if err != nil {
		log.Println(err)
		return retMap
	}
	for _, dev := range devl.Device {
		retMap[dev.Name] = dev.AIN
	}
	return retMap
}
