package fritz

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freepslib"
	log "github.com/sirupsen/logrus"
)

// GetDevices returns a map of all device AINs
func (m *OpFritz) GetDevices(ctx *base.Context) *base.OperatorIO {
	l, err := m.getCachedDeviceList(ctx, true)
	if err != nil {
		return base.MakeOutputError(500, err.Error())
	}
	return base.MakeObjectOutput(l)
}

// DeviceSuggestions returns a map of all device names and AINs
func (m *OpFritz) DeviceSuggestions() map[string]string {
	l, _ := m.getCachedDeviceList(nil, false)
	return l
}

func (m *OpFritz) getCachedDeviceList(ctx *base.Context, forceRefresh bool) (map[string]string, error) {
	devNs := m.getDeviceNamespace()
	devs := devNs.GetAllValues(0)
	if forceRefresh || len(devs) == 0 {
		_, err := m.getDeviceList(ctx)
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
func (m *OpFritz) getDeviceList(ctx *base.Context) (*freepslib.AvmDeviceList, error) {
	devl, err := m.fl.GetDeviceList()
	if err != nil {
		dur := 15 * time.Minute
		ctx.GetLogger().Errorf("Failed to connect to FritzBox to get device list: %v", err)
		m.GE.SetSystemAlert(ctx, "FailedConnection", m.name, 2, fmt.Errorf("Connection to %v failed", m.fc.Address), &dur)
		return nil, err
	}
	m.GE.ResetSystemAlert(ctx, "FailedConnection", m.name)
	devNs := m.getDeviceNamespace()
	for _, dev := range devl.Device {
		devNs.SetValue(dev.AIN, base.MakeObjectOutput(dev), ctx)
		m.checkDeviceForAlerts(ctx, dev)
	}
	return devl, nil
}

// checkDeviceForAlerts set system alerts for certain conditions
func (m *OpFritz) checkDeviceForAlerts(ctx *base.Context, device freepslib.AvmDevice) {
	if device.HKR != nil {
		if device.HKR.Batterylow {
			dur := BatterylowAlertDuration
			m.GE.SetSystemAlert(ctx, "BatteryLow"+device.AIN, m.name, BatterylowSeverity, fmt.Errorf("Battery of %v: %v", device.Name, device.HKR.Battery), &dur)
		} else {
			m.GE.ResetSystemAlert(ctx, "BatteryLow"+device.AIN, m.name)
		}
		if device.HKR.Windowopenactive {
			dur := 15 * time.Minute // TODO: time(device.HKR.Windowopenactiveendtime).Sub(time.Now())
			m.GE.SetSystemAlert(ctx, "WindowOpen"+device.AIN, m.name, WindowOpenSeverity, fmt.Errorf("%v window open", device.Name), &dur)
		} else {
			m.GE.ResetSystemAlert(ctx, "WindowOpen"+device.AIN, m.name)
		}
	}
	if !device.Present {
		dur := 15 * time.Minute
		m.GE.SetSystemAlert(ctx, "DeviceNotPresent"+device.AIN, m.name, DeviceNotPresentSeverity, fmt.Errorf("%v not present", device.Name), &dur)
	} else {
		m.GE.ResetSystemAlert(ctx, "DeviceNotPresent"+device.AIN, m.name)
	}
}
