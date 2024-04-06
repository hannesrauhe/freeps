package fritz

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type Host struct {
	Active             bool
	AddressSource      string
	HostName           string
	IPAddress          string
	InterfaceType      string
	LeaseTimeRemaining uint64
	MACAddress         string
}

func (m *OpFritz) addHost(ctx *base.Context, mac string, ip string, res map[string]interface{}) (Host, error) {
	var host Host
	err := utils.MapToObject(res, &host)
	if err != nil {
		return host, err
	}

	if host.MACAddress == "" {
		host.MACAddress = mac
	}
	if host.IPAddress == "" {
		host.IPAddress = ip
	}

	updFn := func(oldHost base.OperatorIO) *base.OperatorIO {
		if oldHost.IsEmpty() {
			m.executeTrigger(ctx, host, host.MACAddress)
		}
		return base.MakeObjectOutput(host)
	}
	ns := m.getHostsNamespace()
	if host.IPAddress != "" {
		ns.UpdateTransaction("IP:"+host.IPAddress, updFn, ctx.GetID())
	}
	if host.MACAddress != "" {
		ns.UpdateTransaction(host.MACAddress, updFn, ctx.GetID())
	}
	return host, nil
}

// DiscoverHosts retrieves all known hosts from the fritzbox
func (m *OpFritz) DiscoverHosts(ctx *base.Context) *base.OperatorIO {
	res, err := m.fl.GetUpnpDataMap("Hosts", "GetHostNumberOfEntries")
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
		res, err = m.fl.CallUpnpActionWithArgument("Hosts", "GetGenericHostEntry", "NewIndex", newIndex)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error when reading host %v: %v", i, err.Error())
		}
		_, err := m.addHost(ctx, "", "", res)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Cannot parse response %v: %v", i, res)
		}
	}
	return base.MakeSprintfOutput("Discovered %v hosts", numHosts)
}
