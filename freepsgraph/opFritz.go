package freepsgraph

import (
	"log"
	"net/http"
	"sort"

	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
)

type OpFritz struct {
	fl            *freepslib.Freeps
	fc            *freepslib.FBconfig
	cachedDevices map[string]string
}

var _ FreepsOperator = &OpFritz{}

// NewOpFritz creates a new operator for Freeps and Freepsflux
func NewOpFritz(cr *utils.ConfigReader) *OpFritz {
	conf := freepslib.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepslib", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	f, _ := freepslib.NewFreepsLib(&conf)
	return &OpFritz{fl: f, fc: &conf}
}

func (m *OpFritz) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	dev := vars["device"]

	switch fn {
	case "upnp":
		{
			m, err := m.fl.GetUpnpDataMap(vars["serviceName"], vars["actionName"])
			if err == nil {
				return MakeObjectOutput(m)
			}
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getmetrics":
		{
			met, err := m.fl.GetMetrics()
			if err == nil {
				return MakeObjectOutput(&met)
			}
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdevicelistinfos":
		{
			devl, err := m.getDeviceList()
			if err == nil {
				return MakeObjectOutput(devl)
			}
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdata":
		{
			r, err := m.fl.GetData()
			if err == nil {
				return MakeObjectOutput(r)
			}
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "wakeup":
		{
			netdev := vars["netdevice"]
			log.Printf("Waking Up %v", netdev)
			err := m.fl.WakeUpDevice(netdev)
			if err == nil {
				return MakePlainOutput("Woke up %s", netdev)
			}
			return MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	}

	if fn[0:3] == "set" {
		err := m.fl.HomeAutoSwitch(fn, dev, vars)
		if err == nil {
			vars["fn"] = fn
			return MakeObjectOutput(vars)
		}
		return MakeOutputError(http.StatusInternalServerError, err.Error())

	}

	r, err := m.fl.HomeAutomation(fn, dev, vars)
	if err == nil {
		return MakeObjectOutput(r)
	} else {
		return MakeOutputError(http.StatusInternalServerError, err.Error())
	}
}

func (m *OpFritz) GetFunctions() []string {
	swc := m.fl.GetSuggestedSwitchCmds()
	fn := make([]string, 0, len(swc)+1)
	for k := range swc {
		fn = append(fn, k)
	}
	fn = append(fn, "upnp", "getdata", "wakeup")
	sort.Strings(fn)
	return fn
}

func (m *OpFritz) GetPossibleArgs(fn string) []string {
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

func (m *OpFritz) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	if fn == "upnp" {
		ret := map[string]string{}
		if arg == "serviceName" {
			svc, _ := m.fl.GetUpnpServicesShort()
			for _, v := range svc {
				ret[v] = v
			}
			return ret
		} else if arg == "actionName" {
			serviceName, ok := otherArgs["serviceName"]
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

func (m *OpFritz) GetDevices() map[string]string {
	if len(m.cachedDevices) == 0 {
		m.getDeviceList()
	}
	return m.cachedDevices
}

// getDeviceList retrieves the devicelist and caches
func (m *OpFritz) getDeviceList() (*freepslib.AvmDeviceList, error) {
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
