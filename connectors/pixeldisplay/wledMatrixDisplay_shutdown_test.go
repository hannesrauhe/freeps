package pixeldisplay

import (
	"image"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	log "github.com/sirupsen/logrus"
)

/*
Regression tests for the shutdown hang: with an off or unreachable WLED device,
Shutdown used to block forever waiting for the draw loop, so freepsd ignored
/system/shutdown until systemd SIGKILLed it. Shutdown must return in bounded time.
*/

func newTestDisplay(address string) *WLEDMatrixDisplay {
	disp, err := NewWLEDMatrixDisplay(WLEDMatrixDisplayConfig{
		Address:            address,
		Segments:           []WLEDSegmentConfig{{Width: 16, Height: 16, SegID: 0}},
		MinDisplayDuration: time.Millisecond,
		ImageQueueSize:     5,
		MaxAnimationSize:   150,
	})
	if err != nil {
		panic(err)
	}
	return disp
}

func testContext() *base.Context {
	ctx, _ := base.NewBaseContext(log.New())
	return ctx
}

func queueTestImage(t *testing.T, d *WLEDMatrixDisplay) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	out := d.DrawImage(testContext(), img, false)
	if out.IsError() {
		t.Fatalf("DrawImage failed: %v", out.GetError())
	}
}

// TestShutdownWithUnreachableDevice: the draw loop is stuck in a POST that never
// returns; Shutdown must give up on its own deadline instead of blocking forever.
func TestShutdownWithUnreachableDevice(t *testing.T) {
	oldTimeout, oldWait := wledRequestTimeout, shutdownWaitDuration
	defer func() { wledRequestTimeout, shutdownWaitDuration = oldTimeout, oldWait }()
	wledRequestTimeout = 30 * time.Second         // the POST stays stuck...
	shutdownWaitDuration = 300 * time.Millisecond // ...so Shutdown must use its own deadline

	requestStarted := make(chan struct{})
	release := make(chan struct{})
	var once chan struct{}
	once = make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-once:
		default:
			close(once)
			close(requestStarted)
		}
		<-release
	}))
	defer srv.Close()
	defer close(release) // defers run LIFO: release must run first, else srv.Close() waits for the stuck handler

	d := newTestDisplay(srv.URL)
	queueTestImage(t, d)

	select {
	case <-requestStarted: // the draw loop is inside the POST now
	case <-time.After(5 * time.Second):
		t.Fatal("draw loop never contacted the test server")
	}

	done := make(chan struct{})
	go func() {
		d.Shutdown(testContext())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown blocked on an unreachable WLED device")
	}
}

// TestShutdownFastWhenDeviceWorks: with a working device, Shutdown returns
// quickly and the draw loop has really stopped.
func TestShutdownFastWhenDeviceWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	d := newTestDisplay(srv.URL)
	queueTestImage(t, d)
	// give the draw loop a moment to pick the image up
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	d.Shutdown(testContext())
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Shutdown took %v with a working device", elapsed)
	}
	select {
	case <-d.doneChan:
	default:
		t.Fatal("Shutdown returned before the draw loop stopped")
	}
}

// TestShutdownIdempotent: a second Shutdown must not panic on the closed channels.
func TestShutdownIdempotent(t *testing.T) {
	oldWait := shutdownWaitDuration
	defer func() { shutdownWaitDuration = oldWait }()
	shutdownWaitDuration = 300 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	d := newTestDisplay(srv.URL)
	d.Shutdown(testContext())
	d.Shutdown(testContext())
}
