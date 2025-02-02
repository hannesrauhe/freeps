package fritz

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

type Host struct {
	Active             bool
	AddressSource      *string `json:"AddressSource,omitempty"`
	Name               string
	IPAddress          string  `json:"IPAddress,omitempty"`
	InterfaceType      *string `json:"InterfaceType,omitempty"`
	LeaseTimeRemaining uint64
	MACAddress         string `json:"MACAddress,omitempty"`
}

func (o *OpFritz) getHostSensorCategory() string {
	return strings.ReplaceAll(o.name, ".", "_") + "_host"
}

func (o *OpFritz) addHost(ctx *base.Context, byMac string, byIP string, res map[string]interface{}) (Host, error) {
	var host Host

	// Name is the same as HostName, but by using "Name" sensors understands it as the preferred alias
	res["Name"] = res["HostName"]
	delete(res, "HostName")

	err := utils.MapToObject(res, &host)
	if err != nil {
		return host, err
	}
	if host.AddressSource != nil && *host.AddressSource == "" {
		host.AddressSource = nil
	}
	if host.InterfaceType != nil && *host.InterfaceType == "" {
		host.InterfaceType = nil
	}

	if host.MACAddress == "" {
		host.MACAddress = byMac
	}
	if host.IPAddress == "" {
		host.IPAddress = byIP
	}

	opSensor := sensor.GetGlobalSensors()
	if opSensor == nil {
		return host, fmt.Errorf("Sensor integration not available")
	}

	updFn := func(oldHostEntry freepsstore.StoreEntry) *base.OperatorIO {
		activeTag := "active:" + host.MACAddress
		if oldHostEntry == freepsstore.NotFoundEntry {
			if host.Active {
				go o.executeTrigger(ctx, host, activeTag)
			}
		} else {
			var oldHost Host
			err := oldHostEntry.ParseJSON(&oldHost)
			if err != nil {
				dur := ParseErrorDuration
				go o.GE.SetSystemAlert(ctx, "HostParseError", o.name, 2, err, &dur)
				return base.MakeObjectOutput(host)
			}
			if !oldHost.Active && host.Active {
				go o.executeTrigger(ctx, host, activeTag)
			}
			if oldHost.Active && !host.Active {
				go o.executeTrigger(ctx, host, "in"+activeTag)
			}
		}

		return base.MakeObjectOutput(host)
	}
	ns := o.getHostsNamespace()
	sensorName := host.MACAddress
	if sensorName == "" && host.IPAddress != "" { // for VPN devices MAC is unkown
		sensorName = "IP:" + strings.ReplaceAll(host.IPAddress, ".", ":")
	}
	if sensorName != "" { // no identifier, no sensor...
		ns.UpdateTransaction(sensorName, updFn, ctx) // TODO: can be removed in favor of sensors
		err = opSensor.SetSensorPropertyFromFlattenedObject(ctx, o.getHostSensorCategory(), sensorName, host)
	}
	return host, err
}

func (o *OpFritz) getHostByMac(ctx *base.Context, mac string) (*Host, error) {
	res, err := o.fl.CallUpnpActionWithArgument("Hosts", "GetSpecificHostEntry", "NewMACAddress", mac)
	if err != nil {
		return nil, fmt.Errorf("Error when retrieving host for mac %v: %w", mac, err)
	}
	host, err := o.addHost(ctx, mac, "", res)
	if err != nil {
		return nil, fmt.Errorf("Error when adding host for mac %v: %w", mac, err)
	}
	return &host, nil
}

func (o *OpFritz) getHostSuggestions(searchTerm string) map[string]string {
	res := o.getHostsNamespace().GetSearchResultWithMetadata("", searchTerm, "", time.Duration(0), time.Duration(math.MaxInt64))
	macs := map[string]string{}
	for mac, hEntry := range res {
		if utils.StringStartsWith(mac, "IP:") {
			continue
		}
		var h Host
		err := hEntry.ParseJSON(&h)
		if err != nil {
			continue
		}
		macs[fmt.Sprintf("%v (Mac: %v)", h.Name, mac)] = mac
	}
	return macs
}

// DiscoverHosts retrieves all known hosts from the fritzbox
func (o *OpFritz) DiscoverHosts(ctx *base.Context) *base.OperatorIO {
	res, err := o.fl.GetUpnpDataMap("Hosts", "GetHostNumberOfEntries")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error: %v", err.Error())
	}
	numHostsInterface, ok := res["HostNumberOfEntries"]
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "HostNumberOfEntries not found in response: %v", res)
	}
	numHosts, ok := numHostsInterface.(uint64)
	if !ok {
		return base.MakeOutputError(http.StatusInternalServerError, "HostNumberOfEntries not int: %v", res)
	}
	var i uint64 = 0
	for ; i < numHosts; i++ {
		newIndex := fmt.Sprintf("%d", i)
		res, err = o.fl.CallUpnpActionWithArgument("Hosts", "GetGenericHostEntry", "NewIndex", newIndex)
		if err != nil {
			ctx.GetLogger().Errorf("Error when reading host %v: %v", i, err.Error())
			// return base.MakeOutputError(http.StatusInternalServerError, "Error when reading host %v: %v", i, err.Error())
		}
		_, err := o.addHost(ctx, "", "", res)
		if err != nil {
			ctx.GetLogger().Errorf("Cannot parse response of host %v: %v", i, res)
			// return base.MakeOutputError(http.StatusInternalServerError, "Cannot parse response %v: %v", i, res)
		}
	}
	return base.MakeSprintfOutput("Discovered %v hosts", numHosts)
}

func (h *HostArgs) MACAddressSuggestions(otherArgs base.FunctionArguments, o *OpFritz) map[string]string {
	return o.getHostSuggestions(h.MACAddress)
}

type HostArgs struct {
	MACAddress string
}

func (o *OpFritz) IsHostActive(ctx *base.Context, input *base.OperatorIO, args HostArgs) *base.OperatorIO {
	ns := o.getHostsNamespace()
	x := ns.GetValue(args.MACAddress)
	if x.IsError() {
		return x.GetData()
	}
	var h Host
	err := x.ParseJSON(&h)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot parse host entry: %v", err)
	}
	if h.Active {
		return base.MakeEmptyOutput()
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "Host is inactive: %v", h)
}
