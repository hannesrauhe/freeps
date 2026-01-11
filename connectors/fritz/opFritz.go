package fritz

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/connectors/sensor"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freepslib"
)

const maxAge = time.Second * 100
const BatterylowSeverity = 5
const BatterylowAlertDuration = 5 * time.Minute
const WindowOpenSeverity = 2
const DeviceNotPresentSeverity = 3
const DeviceNotPresentAlertDuration = 10 * time.Minute
const AlertDeviceSeverity = 2
const AlertDeviceAlertDuration = 15 * time.Minute
const ParseErrorDuration = 5 * time.Minute
const PollDuration = time.Minute

type OpFritz struct {
	CR                *utils.ConfigReader
	GE                *freepsflow.FlowEngine
	name              string
	fl                *freepslib.Freeps
	fc                *freepslib.FBconfig
	ticker            *time.Ticker
	getDeviceListLock sync.Mutex
}

var _ base.FreepsOperatorWithShutdown = &OpFritz{}
var _ base.FreepsOperatorWithConfig = &OpFritz{}
var _ base.FreepsOperatorWithDynamicFunctions = &OpFritz{}

// GetDefaultConfig returns the default config for the http connector
func (o *OpFritz) GetDefaultConfig() interface{} {
	conf := freepslib.FBconfig{Verbose: false}
	return &conf
}

// InitCopyOfOperator creates a copy of the operator
func (o *OpFritz) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*freepslib.FBconfig)
	if cfg.Address == "" {
		return nil, fmt.Errorf("no address given in config")
	}
	f, err := freepslib.NewFreepsLibWithLogger(cfg, ctx.GetLogger())
	op := &OpFritz{CR: o.CR, GE: o.GE, name: name, fl: f, fc: cfg}
	return op, err
}

// getNetworkDeviceNamespace returns the namespace for the network device cache
func (o *OpFritz) getNetworkDeviceNamespace() freepsstore.StoreNamespace {
	return freepsstore.GetGlobalStore().GetNamespaceNoError("_" + strings.ToLower(o.name) + "_network_devices")
}

// getHostsNamespace returns the namespace for the discovered hosts
func (o *OpFritz) getHostsNamespace() freepsstore.StoreNamespace {
	return freepsstore.GetGlobalStore().GetNamespaceNoError("_" + strings.ToLower(o.name) + "_hosts")
}

// getTemplateNamespace returns the namespace for the template cache
func (o *OpFritz) getTemplateNamespace() freepsstore.StoreNamespace {
	return freepsstore.GetGlobalStore().GetNamespaceNoError("_" + strings.ToLower(o.name) + "_templates")
}

type UpnpArgs struct {
	ServiceName   string
	ActionName    string
	ArgumentName  *string
	ArgumentValue *string
}

func (a *UpnpArgs) ServiceNameSuggestions(otherArgs base.FunctionArguments, o *OpFritz) map[string]string {
	ret := map[string]string{}
	svc, _ := o.fl.GetUpnpServicesShort()
	for _, v := range svc {
		ret[v] = v
	}
	return ret
}

func (a *UpnpArgs) ActionNameSuggestions(otherArgs base.FunctionArguments, o *OpFritz) map[string]string {
	ret := map[string]string{}
	if a.ServiceName == "" {
		return ret
	}
	actions, _ := o.fl.GetUpnpServiceActions(a.ServiceName)
	for _, v := range actions {
		ret[v] = v
	}
	return ret
}

func (a *UpnpArgs) ArgumentNameSuggestions(otherArgs base.FunctionArguments, o *OpFritz) []string {
	if a.ServiceName == "" || a.ActionName == "" {
		return []string{}
	}
	ret, err := o.fl.GetUpnpServiceActionArguments(a.ServiceName, a.ActionName)
	if err != nil {
		return []string{}
	}
	return ret
}

// Upnp executes a function as advertised by the FritzBox via Upnp
func (o *OpFritz) Upnp(ctx *base.Context, input *base.OperatorIO, args UpnpArgs) *base.OperatorIO {
	res := map[string]interface{}{}
	var err error
	if args.ArgumentName == nil || args.ArgumentValue == nil {
		res, err = o.fl.GetUpnpDataMap(args.ServiceName, args.ActionName)
	} else {
		res, err = o.fl.CallUpnpActionWithArgument(args.ServiceName, args.ActionName, *args.ArgumentName, *args.ArgumentValue)
	}
	if err == nil {
		return base.MakeObjectOutput(res)
	}
	pArgs, err2 := o.fl.GetUpnpServiceActionArguments(args.ServiceName, args.ActionName)
	if err2 != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error: %v", err.Error())
	} else {
		return base.MakeOutputError(http.StatusInternalServerError, "Error: %v, Possible Arguments for this Function: %v", err.Error(), pArgs)
	}

}

// ExecuteDynamic executes a dynamic function
func (o *OpFritz) ExecuteDynamic(ctx *base.Context, fn string, args base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	dev := args.Get("device")

	switch fn {
	case "getmetrics":
		{
			met, err := o.fl.GetMetrics()
			if err == nil {
				return base.MakeObjectOutput(&met)
			}
			return base.MakeInternalServerErrorOutput(err)
		}
	case "gettemplates":
		{
			return base.MakeObjectOutput(o.getTemplates())
		}
	case "getdevicelistinfos":
		{
			devl, err := o.getDeviceList(ctx)
			if err == nil {
				return base.MakeObjectOutput(devl)
			}
			return base.MakeInternalServerErrorOutput(err)
		}
	case "getdeviceinfos":
		{
			devObject, err := o.getDeviceByAIN(ctx, dev)
			if err == nil {
				return base.MakeObjectOutput(devObject)
			}
			return base.MakeInternalServerErrorOutput(err)
		}
	case "gettemplatelistinfos":
		{
			tl, err := o.fl.GetTemplateList()
			if err == nil {
				return base.MakeObjectOutput(tl)
			}
			return base.MakeInternalServerErrorOutput(err)
		}
	case "getdata", "getnetdevices":
		{
			r, err := o.fl.GetData()
			if err != nil {
				return base.MakeInternalServerErrorOutput(err)
			}
			netDevNs := o.getNetworkDeviceNamespace()
			for active := range r.Data.Active {
				netDevNs.SetValue(r.Data.Active[active].UID, base.MakeObjectOutput(r.Data.Active[active]), ctx)
			}
			return base.MakeObjectOutput(r)
		}
	case "wakeup":
		{
			netdev := args.Get("netdevice")
			log.Printf("Waking Up %v", netdev)
			err := o.fl.WakeUpDevice(netdev)
			if err == nil {
				return base.MakeSprintfOutput("Woke up %s", netdev)
			}
			return base.MakeInternalServerErrorOutput(err)
		}
	}

	vars := args.GetOriginalCaseMapOnlyFirst()

	if fn[0:3] == "set" {
		err := o.fl.HomeAutoSwitch(fn, dev, vars)
		if err == nil {
			vars["fn"] = fn
			return base.MakeObjectOutput(args)
		}
		return base.MakeInternalServerErrorOutput(err)
	}

	r, err := o.fl.HomeAutomation(fn, dev, vars)
	if err == nil {
		return base.MakeByteOutput(r)
	}
	return base.MakeInternalServerErrorOutput(err)
}

func (o *OpFritz) GetDynamicFunctions() []string {
	swc := o.fl.GetSuggestedSwitchCmds()
	fn := make([]string, 0, len(swc)+1)
	for k := range swc {
		fn = append(fn, k)
	}
	fn = append(fn, "upnp", "getdata", "wakeup", "getmetrics", "getdevices", "getdevicemap", "gettemplates")
	sort.Strings(fn)
	return fn
}

func (o *OpFritz) GetDynamicPossibleArgs(fn string) []string {
	if fn == "upnp" {
		return []string{"serviceName", "actionName"}
	}
	if fn == "wakeup" {
		return []string{"netdevice"}
	}
	swc := o.fl.GetSuggestedSwitchCmds()
	if f, ok := swc[fn]; ok {
		return f
	}
	return make([]string, 0)
}

func (o *OpFritz) GetDynamicArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	if fn == "upnp" {
		ret := map[string]string{}
		if arg == "servicename" {
			svc, _ := o.fl.GetUpnpServicesShort()
			for _, v := range svc {
				ret[v] = v
			}
			return ret
		} else if arg == "actionname" {
			if !otherArgs.Has("serviceName") {
				return ret
			}
			serviceName := otherArgs.Get("serviceName")
			actions, _ := o.fl.GetUpnpServiceActions(serviceName)
			for _, v := range actions {
				ret[v] = v
			}
			return ret
		}
	}
	switch arg {
	case "netdevice":
		ret := map[string]string{}
		nd, err := o.fl.GetData()
		if err != nil || nd == nil {
			return ret
		}
		for _, dev := range nd.Data.Active {
			ret[dev.Name] = dev.UID
		}
		return ret
	case "template":
		return o.getTemplates()
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

// getTemplates returns a map of all templates IDs
func (o *OpFritz) getTemplates() map[string]string {
	tNs := o.getTemplateNamespace()
	keys := tNs.GetAllValues(0)
	if len(keys) == 0 {
		o.getTemplateList()
		keys = tNs.GetAllValues(0)
	}
	r := map[string]string{}
	for ID, cachedTempl := range keys {
		templ, ok := cachedTempl.Output.(freepslib.AvmTemplate)
		if !ok {
			log.Errorf("Cached record for %v is invalid", ID)
			continue
		}
		r[templ.Name] = templ.ID
	}
	return r
}

// GetDeviceMap returns all devices by AIN
func (o *OpFritz) GetDeviceMap(ctx *base.Context) *base.OperatorIO {
	devs, err := o.getDeviceIDs(ctx, false)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	opSensor := sensor.GetGlobalSensors() // cannot be nil, as getDeviceIDs would have returned an error

	r := map[string]freepslib.AvmDevice{}
	for _, sensorName := range devs {
		cachedDev := opSensor.GetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), sensorName, "_internal")
		if cachedDev.IsError() {
			ctx.GetLogger().Errorf("Failed to get sensor entry for %v: %v", sensorName, cachedDev.GetError())
			continue
		}
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			return base.MakeOutputError(http.StatusInternalServerError, "Cached record for %v is invalid", sensorName)
		}
		r[dev.AIN] = dev
	}
	return base.MakeObjectOutput(r)
}

// getTemplateList retrieves the template list and caches
func (o *OpFritz) getTemplateList() (*freepslib.AvmTemplateList, error) {
	templ, err := o.fl.GetTemplateList()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	templNs := o.getTemplateNamespace()
	for _, t := range templ.Template {
		templNs.SetValue(t.ID, base.MakeObjectOutput(t), nil)
	}
	return templ, nil
}

// StartListening starts a loop that continously polls
func (o *OpFritz) StartListening(ctx *base.Context) {
	if o.ticker != nil {
		return
	}
	o.ticker = time.NewTicker(PollDuration)
	go o.loop(ctx)
}

// Shutdown stops the polling loop
func (o *OpFritz) Shutdown(ctx *base.Context) {
	if o.ticker == nil {
		return
	}
	o.ticker.Stop()
	o.ticker = nil
}

func (o *OpFritz) loop(initCtx *base.Context) {
	res := o.DiscoverHosts(initCtx)
	if res.IsError() {
		initCtx.GetLogger().Errorf("Error when discovering hosts: %v", res.GetError())
	}

	if o.ticker == nil {
		return
	}

	for range o.ticker.C {
		start := time.Now()
		ctx := base.CreateContextWithField(initCtx, "component", "Fritz", "Fritz main loop")
		o.getDeviceList(ctx)
		if o.ticker == nil {
			return
		}

		monitorMacs := o.GE.GetTagValues("active", o.name)
		more := o.GE.GetTagValues("inactive", o.name)
		monitorMacs = append(monitorMacs, more...)
		slices.Sort(monitorMacs)
		monitorMacs = slices.Compact(monitorMacs)

		for _, mac := range monitorMacs {
			if o.ticker == nil {
				return
			}
			_, err := o.getHostByMac(ctx, mac)
			if err != nil {
				ctx.GetLogger().Errorf("Cannot monitor host %v: %v", mac, err)
			}
		}
		duration := time.Now().Sub(start)
		if duration > PollDuration {
			//PollDuration is longer than a loop duration, that's a bad sign, create an alert and set the expiration to the duration of that loop (as good as any other value)
			o.GE.SetSystemAlert(ctx, "LongLoopDuration", o.name, 3, fmt.Errorf("Loop ran for %s to monitor %d macs", duration, len(monitorMacs)), &duration)
		}
	}
}
