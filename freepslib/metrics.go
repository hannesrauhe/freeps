package freepslib

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hannesrauhe/freeps/freepslib/fritzbox_upnp"
)

type FritzBoxMetrics struct {
	DeviceModelName      string
	DeviceFriendlyName   string
	Uptime               int64
	BytesReceived        int64 `json:"X_AVM_DE_TotalBytesReceived64",string`
	BytesSent            int64 `json:"X_AVM_DE_TotalBytesSent64",string`
	TransmissionRateUp   int64 `json:"ByteReceiveRate"`
	TransmissionRateDown int64 `json:"ByteSendRate"`
}

func (f *Freeps) getMetricsMap(serviceName string, actionName string) (fritzbox_upnp.Result, error) {
	rmap := fritzbox_upnp.Result{}

	service, ok := f.getService(serviceName)
	if !ok {
		if f.Verbose {
			log.Printf("Available services:\n %v\n", f.metricsObject.Services)
		}
		return rmap, errors.New("cannot find service " + serviceName)
	}
	action, ok := service.Actions[actionName]
	if !ok {
		if f.Verbose {
			log.Printf("Available actions:\n %v\n", service.Actions)
		}
		return rmap, fmt.Errorf("cannot find action %s/%s ", serviceName, actionName)
	}

	rmap, err := action.Call(nil)
	if err != nil {
		return rmap, errors.New("cannot call action " + actionName)
	}
	return rmap, nil
}

func (f *Freeps) initMetrics() error {
	var err error
	if f.metricsObject == nil {
		f.metricsObject, err = fritzbox_upnp.LoadServices("http://"+f.conf.FB_address+":49000", f.conf.FB_user, f.conf.FB_pass, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Freeps) GetMetrics() (FritzBoxMetrics, error) {
	var r FritzBoxMetrics
	err := f.initMetrics()
	if err != nil {
		return r, err
	}
	r.DeviceModelName = f.metricsObject.Device.ModelName
	r.DeviceFriendlyName = f.metricsObject.Device.FriendlyName
	m, err := f.getMetricsMap("urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1", "GetAddonInfos")
	if err != nil {
		return r, err
	}

	m2, err := f.getMetricsMap("urn:schemas-upnp-org:service:WANIPConnection:1", "GetStatusInfo")
	if err != nil {
		return r, err
	}

	for k, v := range m2 {
		m[k] = v
	}

	byt, err := json.Marshal(m)
	if err != nil {
		return r, err
	}
	if f.Verbose {
		log.Printf("Received metrics:\n %q\n", byt)
	}
	err = json.Unmarshal(byt, &r)
	return r, err
}

func (f *Freeps) GetUpnpDataMap(serviceName string, actionName string) (map[string]interface{}, error) {
	rmap := map[string]interface{}{}
	err := f.initMetrics()
	if err != nil {
		return rmap, err
	}

	return f.getMetricsMap(serviceName, actionName)
}

func (f *Freeps) GetUpnpServices() ([]string, error) {
	err := f.initMetrics()
	if err != nil {
		return []string{}, err
	}
	keys := make([]string, 0, len(f.metricsObject.Services))
	for k := range f.metricsObject.Services {
		keys = append(keys, k)
	}

	return keys, nil
}

func (f *Freeps) GetUpnpServicesShort() ([]string, error) {
	err := f.initMetrics()
	if err != nil {
		return []string{}, err
	}
	keys := make([]string, 0, len(f.metricsObject.Services))
	for k := range f.metricsObject.Services {
		keys = append(keys, f.getShortServiceName(k))
	}

	return keys, nil
}

func (f *Freeps) GetUpnpServiceActions(serviceName string) ([]string, error) {
	err := f.initMetrics()
	if err != nil {
		return []string{}, err
	}
	service, ok := f.getService(serviceName)
	if !ok {
		return []string{}, errors.New("cannot find service " + serviceName)
	}

	keys := make([]string, 0, len(service.Actions))
	for k := range service.Actions {
		keys = append(keys, k)
	}

	return keys, nil
}

func (f *Freeps) getShortServiceName(svcName string) string {
	shorts := strings.Split(svcName, ":")
	if len(shorts) < 2 {
		return "INVALID"
	}
	return shorts[len(shorts)-2]
}

// helper function to deal with short service names
func (f *Freeps) getService(svcName string) (*fritzbox_upnp.Service, bool) {
	svc, ok := f.metricsObject.Services[svcName]
	if !ok {
		for k, v := range f.metricsObject.Services {
			if svcName == f.getShortServiceName(k) {
				return v, true
			}
		}
		return nil, false
	}
	return svc, true
}
