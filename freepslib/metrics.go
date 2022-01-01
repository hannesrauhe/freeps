package freepslib

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/hannesrauhe/freeps/freepslib/fritzboxmetrics"
)

type FritzBoxMetrics struct {
	DeviceModelName      string
	DeviceFriendlyName   string
	Uptime               int64
	BytesReceived        int64 `json:"X_AVM_DE_TotalBytesReceived64"`
	BytesSent            int64 `json:"X_AVM_DE_TotalBytesSent64"`
	TransmissionRateUp   int64 `json:"ByteReceiveRate"`
	TransmissionRateDown int64 `json:"ByteSendRate"`
}

func (f *Freeps) getMetricsMap(serviceName string, actionName string) (fritzboxmetrics.Result, error) {
	rmap := fritzboxmetrics.Result{}

	service, ok := f.metricsObject.Services[serviceName]
	if !ok {
		return rmap, errors.New("cannot find service " + serviceName)
	}
	action, ok := service.Actions[actionName]
	if !ok {
		return rmap, errors.New("cannot find service " + actionName)
	}

	rmap, err := action.Call()
	if err != nil {
		return rmap, errors.New("cannot call action " + actionName)
	}
	return rmap, nil
}

func (f *Freeps) GetMetrics() (FritzBoxMetrics, error) {
	var r FritzBoxMetrics
	var err error
	if f.metricsObject == nil {
		f.metricsObject, err = fritzboxmetrics.LoadServices(f.conf.FB_address, uint16(49000), f.conf.FB_user, f.conf.FB_pass)
		if err != nil {
			return r, err
		}
		if f.Verbose {
			log.Printf("Received services:\n %v\n", f.metricsObject.Services)
			log.Printf("Device:\n %v\n", &f.metricsObject.Device)
		}
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
