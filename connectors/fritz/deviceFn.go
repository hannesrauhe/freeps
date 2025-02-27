package fritz

import (
	"fmt"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/hannesrauhe/freepslib"
)

func (o *OpFritz) getDeviceSensorCategory() string {
	return strings.ReplaceAll(o.name, ".", "_") + "_dev"
}

// GetDevices returns a map of all device AINs
func (o *OpFritz) GetDevices(ctx *base.Context) *base.OperatorIO {
	l, err := o.getCachedDevices(ctx, true)
	if err != nil {
		return base.MakeOutputError(500, err.Error())
	}
	return base.MakeObjectOutput(l)
}

// DeviceSuggestions returns a map of all device names and AINs
func (o *OpFritz) DeviceSuggestions() map[string]string {
	l, _ := o.getCachedDevices(nil, false)
	return l
}

// getDeviceIDs returns a list of all device IDs
func (o *OpFritz) getDeviceIDs(ctx *base.Context, forceRefresh bool) ([]string, error) {
	opSensor := sensor.GetGlobalSensors()
	if opSensor == nil {
		return nil, fmt.Errorf("Sensor integration not available")
	}
	devs, err := opSensor.GetSensorNamesInternal(ctx, o.getDeviceSensorCategory())
	if err != nil {
		return nil, err
	}
	if forceRefresh || len(devs) == 0 {
		_, err := o.getDeviceList(ctx)
		if err != nil {
			return nil, err
		}
	}
	return devs, nil
}

// getDeviceByAIN returns the device object for the device with the given AIN
func (o *OpFritz) getDeviceByAIN(ctx *base.Context, AIN string) (*freepslib.AvmDevice, error) {
	devs, err := o.getDeviceIDs(ctx, false)
	if err != nil {
		return nil, err
	}
	opSensor := sensor.GetGlobalSensors() // cannot be nil, as getDeviceIDs would have returned an error
	for _, sensorName := range devs {
		cachedDev := opSensor.GetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), sensorName, "_internal")
		if cachedDev.IsError() {
			ctx.GetLogger().Errorf("Failed to get sensor entry for %v: %v", sensorName, cachedDev.GetError())
			continue
		}
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			ctx.GetLogger().Errorf("Failed to convert sensor entry for %v", sensorName)
			continue
		}
		if dev.AIN == AIN {
			return &dev, nil
		}
	}
	return nil, fmt.Errorf("Device with AIN %v not found", AIN)
}

func (o *OpFritz) getCachedDevices(ctx *base.Context, forceRefresh bool) (map[string]string, error) {
	devs, err := o.getDeviceIDs(ctx, forceRefresh)
	if err != nil {
		return nil, err
	}
	opSensor := sensor.GetGlobalSensors() // cannot be nil, as getDeviceIDs would have returned an error
	r := map[string]string{}

	for _, sensorName := range devs {
		cachedDev := opSensor.GetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), sensorName, "_internal")
		if cachedDev.IsError() {
			ctx.GetLogger().Errorf("Failed to get sensor entry for %v: %v", sensorName, cachedDev.GetError())
			continue
		}
		dev, ok := cachedDev.Output.(freepslib.AvmDevice)
		if !ok {
			ctx.GetLogger().Errorf("Failed to convert sensor entry for %v", sensorName)
			continue
		}
		r[dev.Name] = dev.AIN
	}

	return r, nil
}

func (o *OpFritz) deviceToSensor(ctx *base.Context, device freepslib.AvmDevice) {
	opSensor := sensor.GetGlobalSensors()
	err := opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "_internal", device)
	if err != nil {
		ctx.GetLogger().Errorf("Failed to set sensor property for %v: %v", device.DeviceID, err)
	}
	if device.AIN == "" {
		return
	}
	opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "name", device.Name)
	opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "ain", device.AIN)
	opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "present", device.Present)
	if device.Battery != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "battery", *device.Battery)
	}
	if device.BatteryLow != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), device.DeviceID, "batteryLow", *device.BatteryLow)
	}
	id := device.DeviceID
	if device.EtsiUnitInfo != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "parent", device.EtsiUnitInfo.DeviceID)
	}
	if device.HKR != nil {
		targetTemp, err := utils.ConvertToFloat(device.HKR.Tsoll)
		if err == nil {
			opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "targetTemperature", targetTemp/2)
		}
	}
	if device.Temperature != nil {
		temperature, err := utils.ConvertToFloat(device.Temperature.Celsius)
		if err == nil {
			opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "temperature", temperature/10)
		}
	}
	if device.Powermeter != nil {
		power, err := utils.ConvertToFloat(device.Powermeter.Power)
		if err == nil {
			opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "power", power/1000)
		}
		voltage, err := utils.ConvertToFloat(device.Powermeter.Voltage)
		if err == nil {
			opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "voltage", voltage/1000)
		}
		energy, err := utils.ConvertToFloat(device.Powermeter.Energy)
		if err == nil {
			opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "energy", energy/1000)
		}
	}
	if device.Switch != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "state", device.Switch.State)
	}
	if device.Button != nil {
		t := time.Unix(int64(device.Button.LastPressedTimestamp), 0)
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "lastPressed", t)
	}
	if device.SimpleOnOff != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "state", device.SimpleOnOff.State)
	}
	if device.LevelControl != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "level", device.LevelControl.LevelPercentage)
	}
	if device.ColorControl != nil {
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "hue", device.ColorControl.Hue)
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "saturation", device.ColorControl.Saturation)
		opSensor.SetSensorPropertyInternal(ctx, o.getDeviceSensorCategory(), id, "colorTemp", device.ColorControl.Temperature)
	}

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
	for _, dev := range devl.Device {
		o.deviceToSensor(ctx, dev)
		o.checkDeviceForAlerts(ctx, dev)
	}
	return devl, nil
}

// checkDeviceForAlerts set system alerts for certain conditions
func (o *OpFritz) checkDeviceForAlerts(ctx *base.Context, device freepslib.AvmDevice) {
	deviceId := device.AIN
	if deviceId == "" {
		return
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
	if device.EtsiUnitInfo == nil {
		// no EtsiUnitInfo means that this is a physical device, devices with EtsiUnitInfo are "virtual" sub devices, so they have the same present-state as the physical device
		if !device.Present {
			dur := DeviceNotPresentAlertDuration
			o.GE.SetSystemAlert(ctx, "DeviceNotPresent"+deviceId, o.name, DeviceNotPresentSeverity, fmt.Errorf("%v not present", device.Name), &dur)
		} else {
			o.GE.ResetSystemAlert(ctx, "DeviceNotPresent"+deviceId, o.name)
		}
	}
	if device.Alert != nil {
		if device.Alert.State != 0 {
			dur := AlertDeviceAlertDuration
			o.GE.SetSystemAlert(ctx, "DeviceAlert"+deviceId, o.name, AlertDeviceSeverity, fmt.Errorf("%v is in alert state %v", device.Name, device.Alert.State), &dur)
		} else {
			o.GE.ResetSystemAlert(ctx, "DeviceAlert"+deviceId, o.name)
		}
	}
}
