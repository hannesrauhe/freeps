package fritz

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freepslib"
)

// GetDevices returns a map of all device AINs
func (o *OpFritz) GetDevices(ctx *base.Context) *base.OperatorIO {
	l, err := o.getCachedDeviceList(ctx, true)
	if err != nil {
		return base.MakeOutputError(500, err.Error())
	}
	return base.MakeObjectOutput(l)
}

// DeviceSuggestions returns a map of all device names and AINs
func (o *OpFritz) DeviceSuggestions() map[string]string {
	l, _ := o.getCachedDeviceList(nil, false)
	return l
}

func (o *OpFritz) getCachedDeviceList(ctx *base.Context, forceRefresh bool) (map[string]string, error) {
	devNs := o.getDeviceNamespace()
	devs := devNs.GetAllValues(0)
	if forceRefresh || len(devs) == 0 {
		_, err := o.getDeviceList(ctx)
		if err != nil {
			return nil, err
		}
		devs = devNs.GetAllValues(0)
	}
	r := map[string]string{}

	for AIN, cachedDev := range devs {
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			ctx.GetLogger().Errorf("Cached record for %v is invalid", AIN)
			continue
		}
		r[dev.Name] = dev.AIN
	}

	return r, nil
}

// getDeviceList retrieves the devicelist and caches
func (o *OpFritz) getDeviceList(ctx *base.Context) (*freepslib.AvmDeviceList, error) {
	// lock to prevent multiple calls to getDeviceList, if lock not available return nil
	if !o.getDeviceListLock.TryLock() {
		return nil, fmt.Errorf("getDeviceList already running")
	}
	defer o.getDeviceListLock.Unlock()

	devl, err := o.fl.GetDeviceList()
	if err != nil {
		dur := 15 * time.Minute
		o.GE.SetSystemAlert(ctx, "FailedConnection", o.name, 2, fmt.Errorf("Connection to %v failed", o.fc.Address), &dur)
		return nil, err
	}
	o.GE.ResetSystemAlert(ctx, "FailedConnection", o.name)
	devNs := o.getDeviceNamespace()
	for _, dev := range devl.Device {
		var cachedDevPtr *freepslib.AvmDevice = nil
		cachedValEntry := devNs.GetValue(dev.AIN)
		if cachedValEntry != freepsstore.NotFoundEntry {
			cachedValIo := cachedValEntry.GetData()
			if cachedValIo == nil {
				continue
			}
			if !cachedValIo.IsObject() {
				continue
			}
			cachedDev, ok := cachedValIo.Output.(freepslib.AvmDevice)
			if ok {
				cachedDevPtr = &cachedDev
			}
		}
		devNs.SetValue(dev.AIN, base.MakeObjectOutput(dev), ctx)
		o.checkDeviceForAlerts(ctx, dev, cachedDevPtr)
	}
	return devl, nil
}

// checkDeviceForAlerts set system alerts for certain conditions
func (o *OpFritz) checkDeviceForAlerts(ctx *base.Context, device freepslib.AvmDevice, oldDeviceState *freepslib.AvmDevice) {
	deviceId := device.DeviceID
	if device.EtsiUnitInfo != nil {
		/* make sure alerts are unique per device */
		deviceId = device.EtsiUnitInfo.DeviceID
	}
	if device.HKR != nil {
		if device.HKR.Batterylow {
			dur := BatterylowAlertDuration
			o.GE.SetSystemAlert(ctx, "BatteryLow"+deviceId, o.name, BatterylowSeverity, fmt.Errorf("Battery of %v: %v", device.Name, device.HKR.Battery), &dur)
		} else {
			o.GE.ResetSystemAlert(ctx, "BatteryLow"+deviceId, o.name)
		}
		if device.HKR.Windowopenactive {
			dur := 15 * time.Minute // TODO: time(device.HKR.Windowopenactiveendtime).Sub(time.Now())
			o.GE.SetSystemAlert(ctx, "WindowOpen"+deviceId, o.name, WindowOpenSeverity, fmt.Errorf("%v window open", device.Name), &dur)
		} else {
			o.GE.ResetSystemAlert(ctx, "WindowOpen"+deviceId, o.name)
		}
	}
	if !device.Present {
		dur := DeviceNotPresentAlertDuration
		o.GE.SetSystemAlert(ctx, "DeviceNotPresent"+deviceId, o.name, DeviceNotPresentSeverity, fmt.Errorf("%v not present", device.Name), &dur)
	} else {
		o.GE.ResetSystemAlert(ctx, "DeviceNotPresent"+deviceId, o.name)
	}
	// actual alerts are not grouped by device, but by AIN
	if device.Alert != nil {
		if device.Alert.State != 0 {
			dur := AlertDeviceAlertDuration
			o.GE.SetSystemAlert(ctx, "DeviceAlert"+device.AIN, o.name, AlertDeviceSeverity, fmt.Errorf("%v is in alert state %v", device.Name, device.Alert.State), &dur)
		} else {
			o.GE.ResetSystemAlert(ctx, "DeviceAlert"+device.AIN, o.name)
		}
	}
}
