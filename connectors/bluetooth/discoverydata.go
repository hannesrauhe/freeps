package freepsbluetooth

import (
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

// DiscoveryData is the reduced set of information of Device properties send as input to graphs
type DiscoveryData struct {
	Alias       string
	Address     string
	Name        string
	RSSI        int16
	ServiceData map[string]interface{}
}

func (d *DiscoveryData) Update(change string, value interface{}) (string, error) {
	change = strings.ToLower(change)
	switch change {
	case "name":
		d.Name = value.(string)
	case "ServiceData":
		// TODO(HR)
		// sv1, ok := d.ServiceData[s]
		// if !ok || fmt.Sprint(sv1) != fmt.Sprint(sv1) {
		// 	tags = append(tags, "changed.service:"+s)
		// }
	}
	return change, nil
}

func (d *DiscoveryData) AddServiceData(service string, serviceBytes []byte) string {
	if len(service) > 8 {
		service = service[0:8]
	}

	name := service
	if len(serviceBytes) == 0 {
		d.ServiceData[service] = serviceBytes
		return name
	}

	switch service {
	case "0000180f":
		{
			name = "battery"
			d.ServiceData[name] = int(serviceBytes[0])
		}
	case "0000183b":
		{
			name = "binary"
			d.ServiceData[name] = serviceBytes[0] != 0
		}
	case "00001809":
		{
			name = "temperature"
			d.ServiceData[name] = int(serviceBytes[0])
		}
	default:
		{
			d.ServiceData[name] = serviceBytes
		}
	}

	return name
}

func (fbt *FreepsBluetooth) parseDeviceProperties(prop *device.Device1Properties) *DiscoveryData {
	prop.Lock()
	defer prop.Unlock()
	d := DiscoveryData{Address: prop.Address, Name: prop.Name, Alias: prop.Alias, RSSI: prop.RSSI, ServiceData: map[string]interface{}{}}
	for k, v := range prop.ServiceData {
		dbv, ok := v.(dbus.Variant)
		if !ok {
			fbt.log.Errorf("Service %v data is not dbus.Variant but %T: %v ", k, v, v)
			continue
		}
		serviceBytes, ok := dbv.Value().([]byte)
		if !ok {
			fbt.log.Errorf("Service %v data is not bytes but %T: %v ", k, dbv.Value(), dbv.Value())
			continue
		}
		d.AddServiceData(k, serviceBytes)
	}

	return &d
}

func changedProps(v1, v2 *DiscoveryData, tags []string) []string {
	if v1.Name != v2.Name {
		tags = append(tags, "changed.name")
	}
	if v1.RSSI != v2.RSSI {
		tags = append(tags, "changed.rssi")
	}
	for s, sv2 := range v2.ServiceData {
		sv1, ok := v1.ServiceData[s]
		if !ok || fmt.Sprint(sv2) != fmt.Sprint(sv1) {
			tags = append(tags, "changed.service:"+s)
		}
	}
	return tags
}
