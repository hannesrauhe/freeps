package fritz

import (
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freepslib"
	log "github.com/sirupsen/logrus"
)

// GetDevices returns a map of all device AINs
func (m *OpFritz) GetDevices() *base.OperatorIO {
	l, err := m.getCachedDeviceList(true)
	if err != nil {
		return base.MakeOutputError(500, err.Error())
	}
	return base.MakeObjectOutput(l)
}

func (m *OpFritz) DeviceSuggestions() map[string]string {
	l, _ := m.getCachedDeviceList(false)
	return l
}

func (m *OpFritz) getCachedDeviceList(forceRefresh bool) (map[string]string, error) {
	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	devs := devNs.GetAllValues(0)
	if forceRefresh || len(devs) == 0 {
		_, err := m.getDeviceList()
		if err != nil {
			return nil, err
		}
		devs = devNs.GetAllValues(0)
	}
	r := map[string]string{}

	for AIN, cachedDev := range devs {
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			log.Errorf("Cached record for %v is invalid", AIN)
			continue
		}
		r[dev.Name] = dev.AIN
	}

	return r, nil
}

// getDeviceList retrieves the devicelist and caches
func (m *OpFritz) getDeviceList() (*freepslib.AvmDeviceList, error) {
	devl, err := m.fl.GetDeviceList()
	if err != nil {
		return nil, err
	}
	devNs := freepsstore.GetGlobalStore().GetNamespace(deviceNamespace)
	for _, dev := range devl.Device {
		devNs.SetValue(dev.AIN, base.MakeObjectOutput(dev), "")
	}
	return devl, nil
}
