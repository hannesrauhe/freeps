//go:build !notuya

package tuya

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
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
	IP      string  // IP address of the device (required)
	Version float64 // protocol version, e.g. 3.4
	// DPSNames maps the numeric data point ids reported by the device to
	// sensor property names, e.g. {"6":"humidity", "7":"temperature"}.
	// Unmapped data points are ignored.
	DPSNames map[string]string
}

// TuyaConfig is the config for the tuya operator (section "tuya").
type TuyaConfig struct {
	Enabled bool
	// HeartbeatDuration is how often a keep-alive is sent on the persistent
	// connection; it also bounds how quickly pushed updates are read.
	HeartbeatDuration time.Duration
	// ReconcileDuration is how often the full state is re-queried even
	// though pushes work, to recover from missed frames. Set to 0 to
	// disable; 10-30 minutes is a sensible range.
	ReconcileDuration time.Duration
	// AlertGraceDuration is how long a device has to be continuously
	// unreachable before a DeviceUnreachable alert is raised. Short network
	// blips (the watcher reconnects within one cycle) stay silent. Set to 0
	// for the default of 2 minutes.
	AlertGraceDuration time.Duration
	// SensorCategory is the sensor category the values are written to.
	SensorCategory string
	Devices        map[string]DeviceConfig
}

// OpTuya keeps a persistent connection to each configured Tuya device and
// publishes its data points as sensors whenever the device pushes an update.
// Control commands are sent over the same persistent connection, since a Tuya
// device only accepts one TCP connection at a time.
type OpTuya struct {
	CR       *utils.ConfigReader
	GE       *freepsflow.FlowEngine
	name     string
	config   TuyaConfig
	mu       sync.Mutex
	watchers map[string]*deviceWatcher
}

var _ base.FreepsOperatorWithConfig = &OpTuya{}
var _ base.FreepsOperatorWithShutdown = &OpTuya{}

const (
	defaultHeartbeat      = 12 * time.Second
	defaultReconcile      = 15 * time.Minute
	defaultSensorCategory = "tuya"
	defaultAlertGrace     = 5 * time.Minute
)

// reconnectDelay is a var so that tests can shorten it.
var reconnectDelay = 10 * time.Second

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
		HeartbeatDuration:  defaultHeartbeat,
		ReconcileDuration:  defaultReconcile,
		AlertGraceDuration: defaultAlertGrace,
		SensorCategory:     defaultSensorCategory,
		Devices:            map[string]DeviceConfig{},
	}
}

func (o *OpTuya) InitCopyOfOperator(ctx *base.Context, config interface{}, fullOperatorName string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*TuyaConfig)
	nc := *cfg
	if nc.HeartbeatDuration <= 0 {
		nc.HeartbeatDuration = defaultHeartbeat
	}
	if nc.ReconcileDuration == 0 {
		nc.ReconcileDuration = defaultReconcile
	}
	if nc.SensorCategory == "" {
		nc.SensorCategory = defaultSensorCategory
	}
	if nc.AlertGraceDuration <= 0 {
		nc.AlertGraceDuration = defaultAlertGrace
	}
	// The full config section name ("tuya", or "tuya.<instance>" for a second
	// operator) is kept as the operator name and used as the alert category,
	// following the convention of the fritz operator.
	return &OpTuya{CR: o.CR, GE: o.GE, name: fullOperatorName, config: nc}, nil
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

// updateSensors writes the given DPS values as sensor properties, using the
// device's name mapping. Unmapped and nil values are skipped.
func (o *OpTuya) updateSensors(ctx *base.Context, name string, cfg DeviceConfig, dps map[string]interface{}) error {
	props := propsFromDPS(dps, namesFor(cfg))
	if len(props) == 0 {
		return nil
	}
	gs := sensor.GetGlobalSensors()
	if gs == nil {
		return fmt.Errorf("sensor operator not available, cannot store tuya values")
	}
	if err := gs.SetSensorPropertiesInternal(ctx, o.config.SensorCategory, name, props); err != nil {
		return fmt.Errorf("cannot set sensor properties for tuya device %q: %w", name, err)
	}
	return nil
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

// Status returns the last known data points of a device as reported by the
// persistent connection. It does NOT open a new connection (a Tuya device
// only accepts one at a time); use Query for a forced fresh read.
func (o *OpTuya) Status(ctx *base.Context, mainInput *base.OperatorIO, args StatusArgs) *base.OperatorIO {
	o.mu.Lock()
	w := o.watchers[args.Device]
	o.mu.Unlock()
	if w == nil {
		return base.MakeOutputError(404, "tuya: no watcher for device %q", args.Device)
	}
	dps, connected, lastErr := w.snapshot()
	if len(dps) == 0 {
		return base.MakeOutputError(503, "tuya: no data from %q yet (connected: %v, last error: %v)", args.Device, connected, lastErr)
	}
	return base.MakeObjectOutput(dps)
}

// QueryArgs are the arguments for the Query function.
type QueryArgs struct {
	Device string
}

// Query forces a fresh full query of the device over the persistent
// connection and returns the result. Use Status for the cached values.
func (o *OpTuya) Query(ctx *base.Context, mainInput *base.OperatorIO, args QueryArgs) *base.OperatorIO {
	o.mu.Lock()
	w := o.watchers[args.Device]
	o.mu.Unlock()
	if w == nil {
		return base.MakeOutputError(404, "tuya: no watcher for device %q", args.Device)
	}
	dps, err := w.forceQuery()
	if err != nil {
		return base.MakeOutputError(500, "tuya: %v", err)
	}
	return base.MakeObjectOutput(dps)
}

// SetSwitchArgs are the arguments for the SetSwitch function.
type SetSwitchArgs struct {
	Device string
	On     bool
}

// SetSwitch turns the main switch (data point 1) of a device on or off over
// the persistent connection. The device confirms by pushing the new state,
// which updates the sensors.
func (o *OpTuya) SetSwitch(ctx *base.Context, mainInput *base.OperatorIO, args SetSwitchArgs) *base.OperatorIO {
	return o.setDPSValues(args.Device, map[string]interface{}{"1": args.On})
}

// SetDPSArgs are the arguments for the SetDPS function.
type SetDPSArgs struct {
	Device string
	DPS    string
	Value  string
}

// SetDPS sets an arbitrary data point to a value, which is parsed as bool,
// int or float first and used as string otherwise, e.g. dps=2 value=55 sets
// the target humidity of a dehumidifier to 55%.
func (o *OpTuya) SetDPS(ctx *base.Context, mainInput *base.OperatorIO, args SetDPSArgs) *base.OperatorIO {
	if args.DPS == "" {
		return base.MakeOutputError(400, "tuya: dps argument is required")
	}
	return o.setDPSValues(args.Device, map[string]interface{}{args.DPS: parseDPSValue(args.Value)})
}

// setDPSValues sends a control command for one configured device.
func (o *OpTuya) setDPSValues(device string, dps map[string]interface{}) *base.OperatorIO {
	o.mu.Lock()
	w := o.watchers[device]
	o.mu.Unlock()
	if w == nil {
		return base.MakeOutputError(404, "tuya: no watcher for device %q", device)
	}
	if err := w.Control(dps); err != nil {
		return base.MakeOutputError(500, "tuya: %v", err)
	}
	return base.MakeObjectOutput(dps)
}

// parseDPSValue converts a string argument into the JSON type the data point
// most likely expects: only the literals true/false become a bool (so that a
// numeric data point can still be set to "1"), then integer, then float, and
// anything else stays a string (for enum dps like fan_speed_enum).
func parseDPSValue(s string) interface{} {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true":
		return true
	case "false":
		return false
	}
	if v, err := utils.ConvertToInt64(s); err == nil {
		return v
	}
	if v, err := utils.ConvertToFloat(s); err == nil {
		return v
	}
	return s
}

// ListDevices returns the configured device names with their ids, protocol
// versions and connection state (keys are not returned).
func (o *OpTuya) ListDevices(ctx *base.Context, mainInput *base.OperatorIO) *base.OperatorIO {
	res := map[string]interface{}{}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, n := range o.deviceNames() {
		c := o.config.Devices[n]
		info := map[string]interface{}{
			"id": c.ID, "ip": c.IP, "version": c.Version,
		}
		if w := o.watchers[n]; w != nil {
			dps, connected, lastErr := w.snapshot()
			info["connected"] = connected
			info["lastError"] = fmt.Sprintf("%v", lastErr)
			info["lastValues"] = propsFromDPS(dps, namesFor(c))
		}
		res[n] = info
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

// StartListening starts one watcher goroutine per configured device.
func (o *OpTuya) StartListening(ctx *base.Context) {
	o.mu.Lock()
	if len(o.watchers) > 0 {
		o.mu.Unlock()
		return
	}
	if len(o.config.Devices) == 0 {
		o.mu.Unlock()
		logrus.Info("tuya: no devices configured, connector idle")
		return
	}
	if o.watchers == nil {
		o.watchers = map[string]*deviceWatcher{}
	}
	lctx := base.CreateContextWithField(ctx, "component", "Tuya", "device watcher")
	for name := range o.config.Devices {
		dev, cfg, err := o.deviceFor(name)
		if err != nil {
			ctx.GetLogger().Errorf("tuya: %v", err)
			continue
		}
		if dev.IP == "" {
			ctx.GetLogger().Errorf("tuya device %q has no IP configured", name)
			continue
		}
		w := newWatcher(o, name, dev, cfg,
			base.CreateContextWithField(lctx, "device", name, "tuya device watcher"))
		o.watchers[name] = w
		go w.run()
	}
	o.mu.Unlock()
}

// Shutdown stops all watcher goroutines.
func (o *OpTuya) Shutdown(ctx *base.Context) {
	o.mu.Lock()
	ws := o.watchers
	o.watchers = nil
	o.mu.Unlock()
	for _, w := range ws {
		close(w.stop)
		w.closeConn()
	}
}

// deviceWatcher maintains the persistent connection to one device.
type deviceWatcher struct {
	op   *OpTuya
	name string
	dev  *Device
	cfg  DeviceConfig
	ctx  *base.Context

	stop chan struct{}

	mu        sync.Mutex
	conn      net.Conn
	lastDPS   map[string]interface{}
	connected bool
	lastErr   error

	// firstFail is when the device started to be continuously unreachable
	// (zero while it is known to be working). It implements the alert grace
	// period: short blips that reconnect sooner never alert.
	firstFail time.Time

	queryNow   chan chan error
	controlNow chan controlRequest
}

// controlRequest is a request to send a CONTROL frame on the persistent
// connection; the result is reported back on resp.
type controlRequest struct {
	dps  map[string]interface{}
	resp chan error
}

// newWatcher builds a watcher for the given device.
func newWatcher(op *OpTuya, name string, dev *Device, cfg DeviceConfig, ctx *base.Context) *deviceWatcher {
	return &deviceWatcher{op: op, name: name, dev: dev, cfg: cfg, ctx: ctx,
		stop: make(chan struct{}), queryNow: make(chan chan error, 1),
		controlNow: make(chan controlRequest, 1)}
}

func (w *deviceWatcher) snapshot() (map[string]interface{}, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	copyDPS := make(map[string]interface{}, len(w.lastDPS))
	for k, v := range w.lastDPS {
		copyDPS[k] = v
	}
	return copyDPS, w.connected, w.lastErr
}

func (w *deviceWatcher) closeConn() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn != nil {
		w.conn.Close()
	}
}

// forceQuery asks the running watcher to send a DP_QUERY and waits for the
// next full state (the query response arrives as a normal frame).
func (w *deviceWatcher) forceQuery() (map[string]interface{}, error) {
	if w.queryNow == nil {
		return nil, fmt.Errorf("tuya: watcher for %q not running", w.name)
	}
	resp := make(chan error, 1)
	select {
	case w.queryNow <- resp:
	case <-w.stop:
		return nil, fmt.Errorf("tuya: watcher stopped")
	}
	select {
	case err := <-resp:
		if err != nil {
			return nil, err
		}
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("tuya: query %q timed out", w.name)
	}
	dps, _, err := w.snapshot()
	return dps, err
}

// Control sets data points on the device by asking the running watcher to
// send a CONTROL frame over the persistent connection. It succeeds as soon as
// the frame is written; the device confirms by pushing the new state back,
// which updates the sensors via applyDPS.
func (w *deviceWatcher) Control(dps map[string]interface{}) error {
	if w.controlNow == nil {
		return fmt.Errorf("tuya: watcher for %q not running", w.name)
	}
	resp := make(chan error, 1)
	select {
	case w.controlNow <- controlRequest{dps: dps, resp: resp}:
	case <-w.stop:
		return fmt.Errorf("tuya: watcher stopped")
	}
	select {
	case err := <-resp:
		return err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("tuya: control %q timed out", w.name)
	}
}

func (w *deviceWatcher) run() {
	for {
		err := w.runOnce()
		w.mu.Lock()
		w.connected = false
		w.lastErr = err
		w.mu.Unlock()
		if err != nil {
			w.ctx.GetLogger().Warnf("tuya watcher %q: %v (reconnecting in %v)", w.name, err, reconnectDelay)
			if w.firstFail.IsZero() {
				w.firstFail = time.Now()
			}
			// Only alert once the device has been unreachable for the
			// grace period: wifi blips that reconnect within a cycle or
			// two should not raise an alert. While the device stays down
			// the alert is re-set every cycle, which keeps it alive.
			if time.Since(w.firstFail) >= w.op.config.AlertGraceDuration {
				dur := 3 * reconnectDelay
				w.op.GE.SetSystemAlert(w.ctx, "DeviceUnreachable"+w.name, w.op.name, 2,
					fmt.Errorf("tuya device %q unreachable: %v", w.name, err), &dur)
			}
		}
		select {
		case <-w.stop:
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// runOnce holds one connection until it fails.
func (w *deviceWatcher) runOnce() error {
	conn, sessionKey, err := w.dev.Connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	w.mu.Lock()
	w.conn = conn
	w.connected = true
	w.mu.Unlock()
	w.ctx.GetLogger().Infof("tuya: connected to %q at %s", w.name, w.dev.IP)

	r := &frameReader{conn: conn}
	seqno := uint32(3) // 1 and 2 were used by session negotiation (3.4+)

	// initial full query
	if err := w.dev.SendQuery(conn, sessionKey, seqno); err != nil {
		return fmt.Errorf("initial query: %w", err)
	}
	seqno++
	lastReconcile := time.Now()
	var pendingQuery chan error // set while a forced Query response is expected

	for {
		// a forced Query from the operator side? send DP_QUERY now
		select {
		case resp := <-w.queryNow:
			if err := w.dev.SendQuery(conn, sessionKey, seqno); err != nil {
				resp <- err
			} else {
				seqno++
				if pendingQuery != nil {
					pendingQuery <- fmt.Errorf("tuya: superseded query")
				}
				pendingQuery = resp
			}
		default:
		}
		// a control command from the operator side? send CONTROL now
		select {
		case req := <-w.controlNow:
			if err := w.dev.SendControl(conn, sessionKey, seqno, req.dps); err != nil {
				req.resp <- err
			} else {
				seqno++
				// the write succeeded; the device confirms by pushing the
				// new state back, which updates lastDPS and the sensors
				// via applyDPS. No optimistic local update, so a rejected
				// command cannot diverge from the device until reconcile.
				req.resp <- nil
			}
		default:
		}
		select {
		case <-w.stop:
			return nil
		default:
		}
		// the read deadline doubles as the heartbeat interval
		conn.SetReadDeadline(time.Now().Add(w.op.config.HeartbeatDuration))
		msg, err := r.receive(sessionKey, w.dev.Version)
		if err == errReadTimeout {
			if err := w.dev.SendHeartbeat(conn, sessionKey, seqno); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
			seqno++
			if w.op.config.ReconcileDuration > 0 && time.Since(lastReconcile) >= w.op.config.ReconcileDuration {
				if err := w.dev.SendQuery(conn, sessionKey, seqno); err != nil {
					return fmt.Errorf("reconcile query: %w", err)
				}
				seqno++
				lastReconcile = time.Now()
			}
			continue
		}
		if err != nil {
			return err
		}
		if len(msg.payload) == 0 {
			continue // heartbeat/query ack
		}
		dps, err := w.dev.decodePayload(msg.payload, sessionKey)
		if err != nil {
			w.ctx.GetLogger().Warnf("tuya %q: cannot decode frame: %v", w.name, err)
			continue
		}
		full := msg.cmd == cmdDPQuery
		w.applyDPS(dps, full)
		if full {
			// A complete state is the proof that the device really works
			// again - a TCP connection that dies before answering does not
			// get here, so the grace period is not restarted too early.
			w.mu.Lock()
			w.firstFail = time.Time{}
			w.mu.Unlock()
			w.op.GE.ResetSystemAlert(w.ctx, "DeviceUnreachable"+w.name, w.op.name)
		}
		if full && pendingQuery != nil {
			pendingQuery <- nil
			pendingQuery = nil
		}
	}
}

// applyDPS merges a (possibly partial) DPS update into the last state and
// writes the mapped properties to the sensor system. full=true replaces the
// whole state (query response), otherwise values are merged (push frame).
func (w *deviceWatcher) applyDPS(dps map[string]interface{}, full bool) {
	w.mu.Lock()
	if full || w.lastDPS == nil {
		w.lastDPS = map[string]interface{}{}
	}
	for k, v := range dps {
		w.lastDPS[k] = v
	}
	merged := make(map[string]interface{}, len(w.lastDPS))
	for k, v := range w.lastDPS {
		merged[k] = v
	}
	w.mu.Unlock()

	if err := w.op.updateSensors(w.ctx, w.name, w.cfg, dps); err != nil {
		w.ctx.GetLogger().Warnf("tuya %q: %v", w.name, err)
	}
}
