//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"encoding/hex"
	"fmt"
	"strings"

	"encoding/binary"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

// DiscoveryData is the reduced set of information of Device properties send as input to graphs
type DiscoveryData struct {
	Alias            string
	Address          string
	Name             string
	RSSI             int16
	ServiceData      map[string]interface{}
	ManufacturerData map[uint16]interface{}
}

// Update applies a change to the current state and returns the name of the changed values
func (d *DiscoveryData) Update(change string, value interface{}) ([]string, error) {
	change = strings.ToLower(change)
	changes := []string{"changed." + change}
	conversionSuccess := true
	switch change {
	case "name":
		d.Name, conversionSuccess = value.(string)
	case "alias":
		d.Alias, conversionSuccess = value.(string)
	case "rssi":
		d.RSSI, conversionSuccess = value.(int16)
	case "servicedata":
		oldServiceData := d.ServiceData
		newServiceData, ok := value.(map[string]dbus.Variant)
		if !ok {
			return changes, fmt.Errorf("new Service data is not the expected map type but %T", value)
		}
		d.ServiceData = map[string]interface{}{}
		for service, serviceValue := range newServiceData {
			d.AddServiceData(service, serviceValue)
		}

		for s, sv2 := range oldServiceData {
			sv1, ok := d.ServiceData[s]
			if !ok || fmt.Sprint(sv2) != fmt.Sprint(sv1) {
				changes = append(changes, "changed.service:"+s)
			}
		}
		for s, sv2 := range d.ServiceData {
			sv1, ok := oldServiceData[s]
			if !ok || fmt.Sprint(sv2) != fmt.Sprint(sv1) {
				changes = append(changes, "changed.service:"+s)
			}
		}
	case "manufacturerdata":
		oldManuData := d.ManufacturerData
		newManuData, ok := value.(map[uint16]dbus.Variant)
		if !ok {
			return changes, fmt.Errorf("new Service data is not the expected map type but %T", value)
		}
		d.ManufacturerData = map[uint16]interface{}{}
		for companyId, val := range newManuData {
			d.AddManufacturerData(companyId, val)
		}

		for s, sv2 := range oldManuData {
			sv1, ok := d.ManufacturerData[s]
			if !ok || fmt.Sprint(sv2) != fmt.Sprint(sv1) {
				changes = append(changes, fmt.Sprintf("changed.service:%v", s))
			}
		}
		for s, sv2 := range d.ManufacturerData {
			sv1, ok := oldManuData[s]
			if !ok || fmt.Sprint(sv2) != fmt.Sprint(sv1) {
				changes = append(changes, fmt.Sprintf("changed.service:%v", s))
			}
		}
	}
	if !conversionSuccess {
		return changes, fmt.Errorf("new data for %v is not the expected map type but %T", change, value)
	}
	return changes, nil
}

// AddServiceData converts the value for a given service to the proper type and adds it to the map under the human readable name if available
func (d *DiscoveryData) AddServiceData(service string, v interface{}) (string, error) {
	if len(service) > 8 {
		service = service[0:8]
	}

	dbv, ok := v.(dbus.Variant)
	if !ok {
		return "", fmt.Errorf("Service %v data is not dbus.Variant but %T: %v ", service, v, v)
	}
	serviceBytes, ok := dbv.Value().([]byte)
	if !ok {
		return "", fmt.Errorf("Service %v data is not bytes but %T: %v ", service, dbv.Value(), dbv.Value())
	}

	name := service
	if len(serviceBytes) == 0 {
		d.ServiceData[service] = serviceBytes
		return name, nil
	}

	isUint := false
	switch service {
	case "0000180f":
		{
			name = "battery"
			isUint = true
		}
	case "0000183b":
		{
			name = "binary"
			d.ServiceData[name] = serviceBytes[0] != 0
		}
	case "0000183a":
		{
			name = "magnet"
			isUint = true
		}
	case "00001809":
		{
			name = "temp"
			isUint = true
		}
	default:
		{
			d.ServiceData[name] = serviceBytes
			d.ServiceData[name+"_hex"] = hex.EncodeToString(serviceBytes)
		}
	}
	if isUint {
		var n int
		d.ServiceData[name], n = binary.Uvarint(serviceBytes)
		if n != len(serviceBytes) {
			d.ServiceData[name+"_hex"] = hex.EncodeToString(serviceBytes)
		}
	}

	return name, nil
}

// AddManufacturerData converts the value for a given service to the proper type and adds it to the map under the identifier for the company
func (d *DiscoveryData) AddManufacturerData(companyId uint16, dbv dbus.Variant) error {
	manuBytes, ok := dbv.Value().([]byte)
	if !ok {
		return fmt.Errorf("Data for manufacturer %v data is not bytes but %T: %v ", companyId, dbv.Value(), dbv.Value())
	}

	if len(manuBytes) == 0 {
		d.ManufacturerData[companyId] = manuBytes
		return nil
	}

	switch companyId {
	case 1424:
		{
			d.ManufacturerData[companyId] = string(manuBytes)
		}
	default:
		{
			d.ManufacturerData[companyId] = manuBytes
			// d.ServiceData[name+"_hex"] = hex.EncodeToString(manuBytes)
		}
	}

	return nil
}

func (fbt *FreepsBluetooth) parseDeviceProperties(prop *device.Device1Properties) *DiscoveryData {
	prop.Lock()
	defer prop.Unlock()
	d := DiscoveryData{Address: prop.Address, Name: prop.Name, Alias: prop.Alias, RSSI: prop.RSSI, ServiceData: map[string]interface{}{}}
	for k, v := range prop.ServiceData {
		_, err := d.AddServiceData(k, v)
		if err != nil {
			fbt.log.Errorf("%v", err)
		}
	}

	return &d
}
