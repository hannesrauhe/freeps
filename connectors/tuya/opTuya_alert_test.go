//go:build !notuya

package tuya

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/sirupsen/logrus"
)

// alertRecorder captures the Set/ResetSystemAlert calls the flow engine
// forwards to its hooks.
type alertRecorder struct {
	mu     sync.Mutex
	sets   []string
	resets []string
}

func (r *alertRecorder) OnSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sets = append(r.sets, category+"."+name)
	return nil
}

func (r *alertRecorder) OnResetSystemAlert(ctx *base.Context, name string, category string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resets = append(r.resets, category+"."+name)
	return nil
}

func (r *alertRecorder) snapshot() (sets, resets []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.sets...), append([]string{}, r.resets...)
}

// The alert category must be the full operator (config section) name, like
// the fritz operator does: "tuya" for the base section, "tuya.<instance>"
// for additional ones.
func TestAlertCategoryIsOperatorName(t *testing.T) {
	ctx := base.NewBaseContextWithReason(logrus.New(), "test")
	baseOp := &OpTuya{}
	for _, section := range []string{"tuya", "tuya.lan"} {
		op, err := baseOp.InitCopyOfOperator(ctx, &TuyaConfig{}, section)
		if err != nil {
			t.Fatalf("InitCopyOfOperator(%q): %v", section, err)
		}
		if got := op.(*OpTuya).name; got != section {
			t.Errorf("InitCopyOfOperator(%q): name = %q, want %q", section, got, section)
		}
	}
}

// A device that is briefly unreachable must not raise an alert; only after
// the grace period of continuous unreachability may the alert fire.
func TestUnreachableAlertGracePeriod(t *testing.T) {
	old := reconnectDelay
	reconnectDelay = 20 * time.Millisecond
	defer func() { reconnectDelay = old }()

	_, ge, _ := helper.SetupEngineWithCommonOperators(t, nil)
	rec := &alertRecorder{}
	ge.AddHook(freepsflow.NewFreepsHookWrapper(rec))

	// a port nobody listens on: every connection attempt fails at once
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	op := &OpTuya{
		GE:   ge,
		name: "tuya",
		config: TuyaConfig{
			Enabled:            true,
			HeartbeatDuration:  50 * time.Millisecond,
			AlertGraceDuration: 300 * time.Millisecond,
			SensorCategory:     "tuya",
		},
	}
	dev := &Device{
		ID: "dev1234567890abcdef", Key: "0123456789abcdef", IP: "127.0.0.1",
		Version: 3.3, port: port, dialTimeout: time.Second, readTimeout: time.Second,
	}
	w := newWatcher(op, "ghostdev", dev, DeviceConfig{},
		base.NewBaseContextWithReason(logrus.New(), "test"))
	go w.run()
	defer func() { close(w.stop); w.closeConn() }()

	// several failed reconnect cycles, but still inside the grace period
	time.Sleep(100 * time.Millisecond)
	if sets, _ := rec.snapshot(); len(sets) != 0 {
		t.Fatalf("alert raised before grace period: %v", sets)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		sets, _ := rec.snapshot()
		if len(sets) > 0 {
			want := "tuya.DeviceUnreachableghostdev"
			if sets[0] != want {
				t.Errorf("alert %q, want %q", sets[0], want)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no alert raised after the grace period")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
