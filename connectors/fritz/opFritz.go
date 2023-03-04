package fritz

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
)

const deviceNamespace = "_fritz_devices"
const templateNamespace = "_fritz_templates"
const maxAge = time.Second * 100

type OpFritz struct {
	fl *freepslib.Freeps
	fc *freepslib.FBconfig
}

var _ freepsgraph.FreepsOperator = &OpFritz{}

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
	op := &OpFritz{fl: f, fc: &conf}
	op.getDeviceList() // fill cache
	return op
}

// GetName returns the name of the operator
func (o *OpFritz) GetName() string {
	return "fritz"
}

func (m *OpFritz) Execute(ctx *base.Context, mixedCaseFn string, vars map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	dev := vars["device"]
	fn := strings.ToLower(mixedCaseFn)

	switch fn {
	case "upnp":
		{
			m, err := m.fl.GetUpnpDataMap(vars["serviceName"], vars["actionName"])
			if err == nil {
				return freepsgraph.MakeObjectOutput(m)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getmetrics":
		{
			met, err := m.fl.GetMetrics()
			if err == nil {
				return freepsgraph.MakeObjectOutput(&met)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdevices":
		{
			return freepsgraph.MakeObjectOutput(m.GetDevices())
		}
	case "gettemplates":
		{
			return freepsgraph.MakeObjectOutput(m.GetTemplates())
		}
	case "getdevicelistinfos":
		{
			devl, err := m.getDeviceList()
			if err == nil {
				return freepsgraph.MakeObjectOutput(devl)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdevicemap":
		{
			devl, err := m.GetDeviceMap()
			if err == nil {
				return freepsgraph.MakeObjectOutput(devl)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdeviceinfos":
		{
			devObject, err := m.GetDeviceByAIN(dev)
			if err == nil {
				return freepsgraph.MakeObjectOutput(devObject)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "gettemplatelistinfos":
		{
			tl, err := m.fl.GetTemplateList()
			if err == nil {
				return freepsgraph.MakeObjectOutput(tl)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "getdata":
		{
			r, err := m.fl.GetData()
			if err == nil {
				return freepsgraph.MakeObjectOutput(r)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	case "wakeup":
		{
			netdev := vars["netdevice"]
			log.Printf("Waking Up %v", netdev)
			err := m.fl.WakeUpDevice(netdev)
			if err == nil {
				return freepsgraph.MakePlainOutput("Woke up %s", netdev)
			}
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
	}

	if fn[0:3] == "set" {
		err := m.fl.HomeAutoSwitch(fn, dev, vars)
		if err == nil {
			vars["fn"] = fn
			return freepsgraph.MakeObjectOutput(vars)
		}
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
	}

	r, err := m.fl.HomeAutomation(fn, dev, vars)
	if err == nil {
		return freepsgraph.MakeByteOutput(r)
	}
	return freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
}

func (m *OpFritz) GetFunctions() []string {
	swc := m.fl.GetSuggestedSwitchCmds()
	fn := make([]string, 0, len(swc)+1)
	for k := range swc {
		fn = append(fn, k)
	}
	fn = append(fn, "upnp", "getdata", "wakeup", "getmetrics", "getdevices", "getdevicemap", "gettemplates")
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
	case "template":
		return m.GetTemplates()
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

// GetDevices returns a map of all device AINs
func (m *OpFritz) GetDevices() map[string]string {
	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	keys := devNs.GetKeys()
	if len(keys) == 0 {
		m.getDeviceList()
	}
	keys = devNs.GetKeys()
	r := map[string]string{}
	for _, k := range keys {
		r[k] = k
	}
	return r
}

// GetTemplates returns a map of all templates
func (m *OpFritz) GetTemplates() map[string]string {
	tNs := freepsstore.GetGlobalStore().GetNamespace(templateNamespace)
	keys := tNs.GetKeys()
	if len(keys) == 0 {
		m.getTemplateList()
	}
	keys = tNs.GetKeys()
	r := map[string]string{}
	for _, k := range keys {
		r[k] = k
	}
	return r
}

// GetDeviceByAIN returns the device object for the device with the given AIN
func (m *OpFritz) GetDeviceByAIN(AIN string) (*freepslib.AvmDevice, error) {
	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	cachedDev := devNs.GetValueBeforeExpiration(AIN, maxAge)
	if cachedDev.IsError() {
		devl, err := m.getDeviceList()
		if devl == nil || err != nil {
			return nil, err
		}
		cachedDev = devNs.GetValue(AIN)
	}
	if cachedDev.IsError() {
		return nil, fmt.Errorf("Device with AIN \"%v\" not found", AIN)
	}
	dev, ok := cachedDev.Output.(freepslib.AvmDevice)
	if !ok {
		return nil, fmt.Errorf("Cached record for %v is invalid", AIN)
	}
	return &dev, nil
}

// GetDeviceMap returns all devices by AIN
func (m *OpFritz) GetDeviceMap() (map[string]freepslib.AvmDevice, error) {
	devl, err := m.getDeviceList()
	if devl == nil || err != nil {
		return nil, err
	}
	r := map[string]freepslib.AvmDevice{}

	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	for AIN, cachedDev := range devNs.GetAllValues(0) {
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			return nil, fmt.Errorf("Cached record for %v is invalid", AIN)
		}
		r[AIN] = dev
	}

	return r, nil
}

// getDeviceList retrieves the devicelist and caches
func (m *OpFritz) getDeviceList() (*freepslib.AvmDeviceList, error) {
	devl, err := m.fl.GetDeviceList()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	for _, dev := range devl.Device {
		devNs.SetValue(dev.AIN, freepsgraph.MakeObjectOutput(dev), "")
	}
	return devl, nil
}

// getTemplateList retrieves the template list and caches
func (m *OpFritz) getTemplateList() (*freepslib.AvmTemplateList, error) {
	templ, err := m.fl.GetTemplateList()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	templNs := freepsstore.GetGlobalStore().GetNamespace(templateNamespace)
	for _, t := range templ.Template {
		templNs.SetValue(t.ID, freepsgraph.MakeObjectOutput(t), "")
	}
	return templ, nil
}

// Shutdown (noOp)
func (o *OpFritz) Shutdown(ctx *base.Context) {
}
