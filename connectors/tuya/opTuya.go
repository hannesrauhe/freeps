//go:build !notuya

package tuya

import (
	"fmt"
	"sort"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
)

// DeviceConfig describes a single Tuya device on the LAN.
type DeviceConfig struct {
	ID      string  // device id from the Tuya cloud
	Key     string  // local key (16 chars) from the Tuya cloud
	IP      string  // IP address; empty means "discover via UDP broadcast"
	Version float64 // protocol version, e.g. 3.4
	// DPSNames maps the numeric data point ids reported by the device to
	// sensor property names, e.g. {"6":"humidity", "7":"temperature"}.
	// Unmapped data points are ignored.
	DPSNames map[string]string
}

// TuyaConfig is the config for the tuya operator (section "tuya").
type TuyaConfig struct {
	Enabled bool
	// PollDuration is how often all devices are queried. Keep this >= 10s;
	// Tuya devices only accept one connection at a time and can drop their
	// cloud link when polled too aggressively.
	PollDuration time.Duration
	// SensorCategory is the sensor category the values are written to.
	SensorCategory string
	Devices        map[string]DeviceConfig
}

// OpTuya polls Tuya WiFi devices over the local network and publishes their
// data points as sensors. Read-only: it never sends control commands.
type OpTuya struct {
	CR     *utils.ConfigReader
	GE     *freepsflow.FlowEngine
	name   string
	config TuyaConfig
	ticker *time.Ticker
}

var _ base.FreepsOperatorWithConfig = &OpTuya{}
var _ base.FreepsOperatorWithShutdown = &OpTuya{}

const defaultPollDuration = time.Minute
const defaultSensorCategory = "tuya"

// defaultDPSNames are data point ids that are common enough to not need
// configuration. Device-specific codes should be added in the config.
var defaultDPSNames = map[string]string{
	"1":   "switch",
	"2":   "setHumidity",
	"4":   "fanSpeed",
	"5":   "workMode",
	"6":   "humidity",
	"7":   "temperature",
	"10":  "anion",
	"16":  "childLock",
	"17":  "countdown",
	"19":  "fault",
	"23":  "filterLife",
	"24":  "tempUnit",
	"101": "switch",
}

func (o *OpTuya) GetDefaultConfig() interface{} {
	return &TuyaConfig{
		PollDuration:   defaultPollDuration,
		SensorCategory: defaultSensorCategory,
		Devices:        map[string]DeviceConfig{},
	}
}

func (o *OpTuya) InitCopyOfOperator(ctx *base.Context, config interface{}, fullOperatorName string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*TuyaConfig)
	nc := *cfg
	if nc.PollDuration <= 0 {
		nc.PollDuration = defaultPollDuration
	}
	if nc.SensorCategory == "" {
		nc.SensorCategory = defaultSensorCategory
	}
	// instance name from a dotted section, e.g. "tuya.lan" -> "lan"
	name := fullOperatorName
	if len(name) > len("tuya.") && name[:5] == "tuya." {
		name = name[5:]
	} else {
		name = "default"
	}
	return &OpTuya{CR: o.CR, GE: o.GE, name: name, config: nc}, nil
}

// deviceNames returns the configured device names, sorted for stable output.
func (o *OpTuya) deviceNames() []string {
	names := make([]string, 0, len(o.config.Devices))
	for n := range o.config.Devices {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (o *OpTuya) deviceFor(name string) (*Device, DeviceConfig, error) {
	cfg, ok := o.config.Devices[name]
	if !ok {
		return nil, cfg, fmt.Errorf("tuya device %q not configured, known devices: %v", name, o.deviceNames())
	}
	ver := cfg.Version
	if ver == 0 {
		ver = 3.3
	}
	dev := &Device{ID: cfg.ID, Key: cfg.Key, IP: cfg.IP, Version: ver}
	return dev, cfg, nil
}

// namesFor returns the DPS id -> sensor property name mapping for a device,
// merging the built-in defaults with the device specific config.
func namesFor(cfg DeviceConfig) map[string]string {
	m := make(map[string]string, len(defaultDPSNames)+len(cfg.DPSNames))
	for k, v := range defaultDPSNames {
		m[k] = v
	}
	for k, v := range cfg.DPSNames {
		m[k] = v
	}
	return m
}

// propsFromDPS converts the raw DPS map into sensor properties, using the
// name mapping. Unmapped and nil values are skipped.
func propsFromDPS(dps map[string]interface{}, names map[string]string) map[string]interface{} {
	props := map[string]interface{}{}
	for id, name := range names {
		v, ok := dps[id]
		if !ok || v == nil {
			continue
		}
		props[name] = v
	}
	return props
}

// pollDevice queries one device and writes its data points as sensor
// properties. Returns the properties that were written.
func (o *OpTuya) pollDevice(ctx *base.Context, name string) (map[string]interface{}, error) {
	dev, cfg, err := o.deviceFor(name)
	if err != nil {
		return nil, err
	}
	if dev.IP == "" {
		return nil, fmt.Errorf("tuya device %q has no IP configured", name)
	}
	dps, err := dev.Status()
	if err != nil {
		return nil, err
	}
	props := propsFromDPS(dps, namesFor(cfg))
	if len(props) == 0 {
		return nil, fmt.Errorf("tuya device %q reported dps %v but none are mapped to names", name, keysOf(dps))
	}
	gs := sensor.GetGlobalSensors()
	if gs == nil {
		return nil, fmt.Errorf("sensor operator not available, cannot store tuya values")
	}
	if err := gs.SetSensorPropertiesInternal(ctx, o.config.SensorCategory, name, props); err != nil {
		return nil, fmt.Errorf("cannot set sensor properties for tuya device %q: %w", name, err)
	}
	return props, nil
}

func keysOf(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// StatusArgs are the arguments for the Status function.
type StatusArgs struct {
	Device string
}

// Status queries a single Tuya device over the LAN and returns its data
// points, both raw (by DPS id) and mapped to sensor property names. The
// values are also stored as sensors.
func (o *OpTuya) Status(ctx *base.Context, mainInput *base.OperatorIO, args StatusArgs) *base.OperatorIO {
	props, err := o.pollDevice(ctx, args.Device)
	if err != nil {
		ctx.GetLogger().Warnf("tuya status for %q failed: %v", args.Device, err)
		return base.MakeOutputError(500, "tuya: %v", err)
	}
	return base.MakeObjectOutput(props)
}

// ListDevices returns the configured device names with their ids and
// protocol versions (keys are not returned).
func (o *OpTuya) ListDevices(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	res := map[string]interface{}{}
	for _, n := range o.deviceNames() {
		c := o.config.Devices[n]
		res[n] = map[string]interface{}{
			"id": c.ID, "ip": c.IP, "version": c.Version,
			"dpsNames": namesFor(c),
		}
	}
	return base.MakeObjectOutput(res)
}

// DevicesSuggestions is used by the UI to suggest device names.
func (o *OpTuya) DevicesSuggestions(ctx *base.Context) *base.OperatorIO {
	res := map[string]string{}
	for _, n := range o.deviceNames() {
		res[n] = n
	}
	return base.MakeObjectOutput(res)
}

func (o *OpTuya) loop(initCtx *base.Context) {
	o.pollAll(initCtx)
	if o.ticker == nil {
		return
	}
	for range o.ticker.C {
		if o.ticker == nil {
			return
		}
		start := time.Now()
		ctx := base.CreateContextWithField(initCtx, "component", "Tuya", "periodic poll")
		o.pollAll(ctx)
		if o.ticker == nil {
			return
		}
		d := time.Since(start)
		if d > o.config.PollDuration {
			o.GE.SetSystemAlert(ctx, "LongLoopDuration", o.name, 3,
				fmt.Errorf("tuya poll loop ran for %s", d), &d)
		}
	}
}

// pollAll queries every configured device. Failures per device are logged
// and turned into an alert, they never stop the loop.
func (o *OpTuya) pollAll(ctx *base.Context) {
	if len(o.config.Devices) == 0 {
		return
	}
	failures := []string{}
	for _, name := range o.deviceNames() {
		if o.ticker == nil {
			return
		}
		if _, err := o.pollDevice(ctx, name); err != nil {
			ctx.GetLogger().Warnf("tuya poll %q: %v", name, err)
			failures = append(failures, name)
		}
	}
	if len(failures) > 0 {
		dur := o.config.PollDuration * 3
		o.GE.SetSystemAlert(ctx, "DeviceUnreachable", o.name, 2,
			fmt.Errorf("tuya device(s) not reachable: %v", failures), &dur)
	} else {
		o.GE.ResetSystemAlert(ctx, "DeviceUnreachable", o.name)
	}
}

// StartListening starts the polling loop.
func (o *OpTuya) StartListening(ctx *base.Context) {
	if o.ticker != nil {
		return
	}
	if len(o.config.Devices) == 0 {
		logrus.Info("tuya: no devices configured, polling disabled")
		return
	}
	o.ticker = time.NewTicker(o.config.PollDuration)
	go o.loop(ctx)
}

// Shutdown stops the polling loop.
func (o *OpTuya) Shutdown(ctx *base.Context) {
	if o.ticker == nil {
		return
	}
	o.ticker.Stop()
	o.ticker = nil
}
