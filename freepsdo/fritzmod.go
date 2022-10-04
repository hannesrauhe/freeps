package freepsdo

import (
	"encoding/json"
	"net/http"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
)

type FritzMod struct {
	fl            *freepslib.Freeps
	ff            *freepsflux.FreepsFlux
	fc            *freepslib.FBconfig
	ffc           *freepsflux.FreepsFluxConfig
	cachedDevices map[string]string
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
	dev := vars["device"]

	if fn == "upnp" {
		m, err := m.fl.GetUpnpDataMap(vars["serviceName"], vars["actionName"])
		if err == nil {
			jrw.WriteSuccessMessage(m)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	} else if fn == "freepsflux" {
		err = m.ff.Push()
		if err == nil {
			jrw.WriteSuccess()
		} else {
			jrw.WriteError(http.StatusInternalServerError, "Freepsflux error when pushing: %v", err.Error())
		}
	} else if fn == "getdevicelistinfos" {
		devl, err := m.getDeviceList()
		if err == nil {
			jrw.WriteSuccessMessage(devl)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	} else if fn == "getdata" {
		r, err := m.fl.GetData()
		if err == nil {
			jrw.WriteSuccessMessage(r)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	} else if fn == "wakeup" {
		netdev := vars["netdevice"]
		log.Printf("Waking Up %v", netdev)
		err = m.fl.WakeUpDevice(netdev)
		if err == nil {
			jrw.WriteSuccessf("Woke up %s", netdev)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	} else if fn[0:3] == "set" {
		err = m.fl.HomeAutoSwitch(fn, dev, vars)
		if err == nil {
			vars["fn"] = fn
			jrw.WriteSuccessMessage(vars)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	} else {
		r, err := m.fl.HomeAutomation(fn, dev, vars)
		if err == nil {
			jrw.WriteSuccessMessage(r)
		} else {
			jrw.WriteError(http.StatusInternalServerError, err.Error())
		}
	}
}

func (m *FritzMod) GetFunctions() []string {
	swc := m.fl.GetSuggestedSwitchCmds()
	fn := make([]string, 0, len(swc)+1)
	for k := range swc {
		fn = append(fn, k)
	}
	fn = append(fn, "upnp", "getdata", "wakeup")
	sort.Strings(fn)
	return fn
}

func (m *FritzMod) GetPossibleArgs(fn string) []string {
	if fn == "upnp" {
		return []string{"serviceName", "actionName"}
	}
	if fn == "wakeup" {
		return []string{"netdevice"}
	}
	swc := m.fl.GetSuggestedSwitchCmds()
	if f, ok := swc[fn]; ok {
		return f
	}
	return make([]string, 0)
}

func (m *FritzMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	if fn == "upnp" {
		ret := map[string]string{}
		if arg == "serviceName" {
			svc, _ := m.fl.GetUpnpServicesShort()
			for _, v := range svc {
				ret[v] = v
			}
			return ret
		} else if arg == "actionName" {
			serviceName, ok := otherArgs["serviceName"].(string)
			if !ok {
				return ret
			}
			actions, _ := m.fl.GetUpnpServiceActions(serviceName)
			for _, v := range actions {
				ret[v] = v
			}
			return ret
		}
	}
	switch arg {
	case "netdevice":
		ret := map[string]string{}
		nd, err := m.fl.GetData()
		if err != nil || nd == nil {
			return ret
		}
		for _, dev := range nd.Data.Active {
			ret[dev.Name] = dev.UID
		}
		return ret
	case "device":
		return m.GetDevices()
	case "onoff":
		return map[string]string{"On": "1", "Off": "0", "Toggle": "2"}
	case "param":
		return map[string]string{"Off": "253", "16": "32", "18": "36", "20": "40", "22": "44", "24": "48"}
	case "temperature": // fn=="setcolortemperature"
		return map[string]string{"2700K": "2700", "3500K": "3500", "4250K": "4250", "5000K": "5000", "6500K": "6500"}
	case "level":
		if fn == "setlevel" {
			return map[string]string{"50": "50", "100": "100", "150": "150", "200": "200", "255": "255"}
		}
		return map[string]string{"5": "5", "25": "25", "50": "50", "75": "75", "100": "100"}
	case "duration":
		return map[string]string{"0": "0", "0.1s": "1", "1s": "10"}
	case "hue":
		return map[string]string{"red": "358"}
	case "saturation":
		return map[string]string{"red": "180"}
	}
	return map[string]string{}
}

func (m *FritzMod) GetDevices() map[string]string {
	if len(m.cachedDevices) == 0 {
		m.getDeviceList()
	}
	return m.cachedDevices
}

// getDeviceList retrieves the devicelist and caches
func (m *FritzMod) getDeviceList() (*freepslib.AvmDeviceList, error) {
	m.cachedDevices = map[string]string{}
	devl, err := m.fl.GetDeviceList()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, dev := range devl.Device {
		m.cachedDevices[dev.Name] = dev.AIN
	}
	return devl, nil
}
