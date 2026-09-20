//go:build !notuya

package tuya

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"github.com/sirupsen/logrus"
)

// fakeDevice answers every frame with a 28-byte ack and records the data
// points of received CONTROL (cmd 7) frames. Protocol 3.3 framing (CRC32,
// no session negotiation), like the smart plug on the LAN.
type fakeDevice struct {
	t        *testing.T
	listener net.Listener
	key      []byte
	controls chan map[string]interface{}
}

func newFakeDevice(t *testing.T, key []byte) *fakeDevice {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeDevice{t: t, listener: l, key: key, controls: make(chan map[string]interface{}, 8)}
	go f.acceptLoop()
	return f
}

func (f *fakeDevice) port() int {
	return f.listener.Addr().(*net.TCPAddr).Port
}

func (f *fakeDevice) acceptLoop() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.serve(conn)
	}
}

func (f *fakeDevice) serve(conn net.Conn) {
	defer conn.Close()
	r := &frameReader{conn: conn}
	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		frame, cmd, err := r.nextRawFrame()
		if err != nil {
			return
		}
		if cmd == cmdControl {
			// request body: 15-byte version header + AES(payload), no retcode
			body := frame[16 : len(frame)-8]
			if len(body) < 16 || body[3] != 0 {
				f.t.Errorf("fake device: control frame has no version header: %x", body[:min(16, len(body))])
				return
			}
			plain, err := aesECBDecrypt(f.key, body[15:])
			if err != nil {
				f.t.Errorf("fake device: cannot decrypt control: %v", err)
				return
			}
			var p struct {
				DPS map[string]interface{} `json:"dps"`
			}
			if err := json.Unmarshal(plain, &p); err != nil {
				f.t.Errorf("fake device: control payload not JSON: %v (%s)", err, plain)
				return
			}
			f.controls <- p.DPS
		}
		// ack: retcode 0, empty payload -> 28 byte frame
		ack, err := pack55AA(0, cmd, []byte{0, 0, 0, 0}, nil)
		if err != nil {
			return
		}
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		if _, err := conn.Write(ack); err != nil {
			return
		}
	}
}

// nextRawFrame reads one complete 55AA frame and returns the raw bytes plus
// the command code. Unlike frameReader.receive it does not strip the retcode,
// so request payloads (which have none) can be decoded correctly.
func (r *frameReader) nextRawFrame() ([]byte, uint32, error) {
	for {
		if err := r.fill(minFrameLen); err != nil {
			return nil, 0, err
		}
		prefix, _, cmd, total, err := parseHeader(r.buf)
		if err != nil {
			r.buf = r.buf[1:]
			continue
		}
		if prefix != prefix55AA {
			return nil, 0, errNot55AA
		}
		if err := r.fill(total); err != nil {
			return nil, 0, err
		}
		frame := make([]byte, total)
		copy(frame, r.buf[:total])
		r.buf = r.buf[total:]
		return frame, cmd, nil
	}
}

var errNot55AA = errors.New("tuya test: not a 55AA frame")

func TestWatcherControl(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	key := []byte("0123456789abcdef")
	fake := newFakeDevice(t, key)
	defer fake.listener.Close()

	op := &OpTuya{
		GE:   ge,
		name: "test",
		config: TuyaConfig{
			Enabled:           true,
			HeartbeatDuration: 200 * time.Millisecond,
			SensorCategory:    "tuya",
		},
	}
	dev := &Device{
		ID: "dev1234567890abcdef", Key: string(key), IP: "127.0.0.1",
		Version: 3.3, port: fake.port(),
		dialTimeout: time.Second, readTimeout: time.Second,
	}
	w := newWatcher(op, "testdev", dev, DeviceConfig{},
		base.NewBaseContextWithReason(logrus.New(), "test"))
	go w.run()
	defer func() { close(w.stop); w.closeConn() }()

	// wait until the watcher is connected and has queried the device
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, connected, _ := w.snapshot(); connected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watcher did not connect to fake device")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := w.Control(map[string]interface{}{"1": true}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	select {
	case dps := <-fake.controls:
		if dps["1"] != true {
			t.Errorf("fake device got dps %v, want 1=true", dps)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("fake device never received a CONTROL frame")
	}

	// the operator-level SetSwitch must reach the same watcher
	op.mu.Lock()
	op.watchers = map[string]*deviceWatcher{"testdev": w}
	op.mu.Unlock()
	res := op.SetSwitch(ctx, base.MakeEmptyOutput(), SetSwitchArgs{Device: "testdev", On: false})
	if res.IsError() {
		t.Fatalf("SetSwitch: %v", res.GetError())
	}
	select {
	case dps := <-fake.controls:
		if dps["1"] != false {
			t.Errorf("fake device got dps %v, want 1=false", dps)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SetSwitch did not reach the fake device")
	}
}

// TestWatcherControlNotRunning checks the error path when no connection exists.
func TestWatcherControlNotRunning(t *testing.T) {
	op := &OpTuya{name: "test"}
	w := newWatcher(op, "ghost", &Device{ID: "x", Key: "0123456789abcdef", Version: 3.3},
		DeviceConfig{}, base.NewBaseContextWithReason(logrus.New(), "test"))
	w.controlNow = nil
	if err := w.Control(map[string]interface{}{"1": true}); err == nil {
		t.Error("expected an error when the watcher is not running")
	}
}

// TestSetSwitchNoWatcher checks the error for an unknown device.
func TestSetSwitchNoWatcher(t *testing.T) {
	op := &OpTuya{name: "test", config: TuyaConfig{SensorCategory: "tuya"}}
	res := op.SetSwitch(base.NewBaseContextWithReason(logrus.New(), "test"),
		base.MakeEmptyOutput(), SetSwitchArgs{Device: "nope", On: true})
	if res == nil || !res.IsError() {
		t.Fatalf("expected an error, got %v", res)
	}
}

// TestSetSwitchIdempotent checks that a control command is only sent when the
// device does not already report the requested value: the dehumidifier beeps
// on every command, so re-sending a state it is already in is audible noise.
func TestSetSwitchIdempotent(t *testing.T) {
	ctx, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	key := []byte("0123456789abcdef")
	fake := newFakeDevice(t, key)
	defer fake.listener.Close()

	op := &OpTuya{
		GE:   ge,
		name: "test",
		config: TuyaConfig{
			Enabled:           true,
			HeartbeatDuration: 200 * time.Millisecond,
			SensorCategory:    "tuya",
		},
	}
	dev := &Device{
		ID: "dev1234567890abcdef", Key: string(key), IP: "127.0.0.1",
		Version: 3.3, port: fake.port(),
		dialTimeout: time.Second, readTimeout: time.Second,
	}
	w := newWatcher(op, "testdev", dev, DeviceConfig{},
		base.NewBaseContextWithReason(logrus.New(), "test"))
	go w.run()
	defer func() { close(w.stop); w.closeConn() }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, connected, _ := w.snapshot(); connected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watcher did not connect to fake device")
		}
		time.Sleep(20 * time.Millisecond)
	}
	op.mu.Lock()
	op.watchers = map[string]*deviceWatcher{"testdev": w}
	op.mu.Unlock()

	// simulate the device reporting switch=true
	w.applyDPS(map[string]interface{}{"1": true}, true)

	// setting it to true again must NOT send a command (no beep)
	if res := op.SetSwitch(ctx, base.MakeEmptyOutput(), SetSwitchArgs{Device: "testdev", On: true}); res.IsError() {
		t.Fatalf("SetSwitch(true) errored: %v", res.GetError())
	}
	select {
	case dps := <-fake.controls:
		t.Errorf("redundant SetSwitch(true) sent a command: %v", dps)
	case <-time.After(500 * time.Millisecond):
		// good: nothing sent
	}

	// setting it to false is a real change and must send
	if res := op.SetSwitch(ctx, base.MakeEmptyOutput(), SetSwitchArgs{Device: "testdev", On: false}); res.IsError() {
		t.Fatalf("SetSwitch(false) errored: %v", res.GetError())
	}
	select {
	case dps := <-fake.controls:
		if dps["1"] != false {
			t.Errorf("fake device got dps %v, want 1=false", dps)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("real change did not send a CONTROL frame")
	}
}

// TestDPSAlreadySet covers the value comparison, incl. int/float equivalence
// (the device reports JSON numbers as float64) and unseen data points.
func TestDPSAlreadySet(t *testing.T) {
	last := map[string]interface{}{"1": true, "2": float64(65), "6": float64(48)}
	cases := []struct {
		name string
		want map[string]interface{}
		set  bool
	}{
		{"same bool", map[string]interface{}{"1": true}, true},
		{"diff bool", map[string]interface{}{"1": false}, false},
		{"int vs float equal", map[string]interface{}{"2": int64(65)}, true},
		{"int vs float diff", map[string]interface{}{"2": int64(55)}, false},
		{"unseen dps", map[string]interface{}{"99": true}, false},
		{"multi all match", map[string]interface{}{"1": true, "6": 48}, true},
		{"multi one diff", map[string]interface{}{"1": true, "6": 49}, false},
	}
	for _, c := range cases {
		if got := dpsAlreadySet(last, c.want); got != c.set {
			t.Errorf("%s: dpsAlreadySet = %v, want %v", c.name, got, c.set)
		}
	}
}
