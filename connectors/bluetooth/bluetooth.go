//go:build !nobluetooth

package freepsbluetooth

import (
	"fmt"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	"context"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/sirupsen/logrus"
)

var btwatcher *FreepsBluetooth

// FreepsBluetooth provides options to scan for bt devices and execute operations based on that
type FreepsBluetooth struct {
	config             *BluetoothConfig
	discoInitLock      sync.Mutex
	cancel             context.CancelFunc
	shuttingDown       bool
	nextIterationTimer *time.Timer
	log                logrus.FieldLogger
	ge                 *freepsgraph.GraphEngine
	monitors           *monitors
}

// NewBTWatcher creates a new BT watcher
func NewBTWatcher(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*FreepsBluetooth, error) {
	btc := defaultBluetoothConfig
	err := cr.ReadSectionWithDefaults("bluetooth", &btc)
	if err != nil {
		return nil, err
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		logrus.Print(err)
	}

	btlogger := logger.WithField("component", "bluetooth")
	if !btc.Enabled {
		btlogger.Infof("Bluetooth module disabled in config")
		return nil, nil
	}
	btwatcher = &FreepsBluetooth{config: &btc, log: btlogger, shuttingDown: false, ge: ge, monitors: &monitors{watchers: map[string]deviceEntry{}}}

	err = btwatcher.StartDiscovery()
	if err != nil {
		btwatcher = nil
		api.Exit()
	}
	return btwatcher, err
}

// StartDiscovery starts the process of discovery new bluetooth devices
func (fbt *FreepsBluetooth) StartDiscovery() error {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if btwatcher.shuttingDown {
		return nil
	}

	if fbt.cancel != nil {
		return nil
	}
	err := btwatcher.run(fbt.config.AdapterName, false)
	if err != nil {
		return err
	}
	fbt.nextIterationTimer = time.AfterFunc(fbt.config.DiscoveryDuration, func() {
		fbt.StopDiscovery(false)
	})
	return err
}

// StopDiscovery stops bluetooth discovery and schedules the next discovery process
func (fbt *FreepsBluetooth) StopDiscovery(restartImmediately bool) {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if btwatcher.shuttingDown {
		return
	}

	if fbt.cancel == nil {
		return
	}

	fbt.log.Infof("Stopping discovery, immediate restart: %v", restartImmediately)
	fbt.cancel()
	fbt.cancel = nil
	dur := fbt.config.DiscoveryPauseDuration
	if restartImmediately {
		dur = time.Second
	}
	fbt.nextIterationTimer = time.AfterFunc(dur, func() {
		err := fbt.StartDiscovery()
		if err != nil {
			fbt.log.Errorf("Failed to Start Discovery")
		}
	})
}

// Shutdown stops discovery and does not schedule it again
func (fbt *FreepsBluetooth) Shutdown() {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	fbt.shuttingDown = true

	if fbt.nextIterationTimer != nil {
		fbt.nextIterationTimer.Stop()
	}

	if fbt.cancel != nil {
		fbt.cancel()
		fbt.cancel = nil
	}

	fbt.deleteAllMonitors()

	api.Exit()
}

func (fbt *FreepsBluetooth) run(adapterID string, onlyBeacon bool) error {
	a, err := adapter.GetAdapter(adapterID)
	if err != nil {
		return err
	}

	fbt.log.Debug("Flush cached devices")
	err = a.FlushDevices()
	if err != nil {
		return err
	}
	freepsstore.GetGlobalStore().GetNamespace("_bluetooth").DeleteOlder(fbt.config.ForgetDeviceDuration)

	discovery, cancel, err := api.Discover(a, nil)
	if err != nil {
		return err
	}
	fbt.cancel = cancel

	go func() {
		fbt.log.Debug("Started discovery")
		watchRequests := fbt.ge.GetTagValues("device")
		for ev := range discovery {

			if ev.Type == adapter.DeviceRemoved {
				continue
			}

			dev, err := device.NewDevice1(ev.Path)
			if err != nil {
				fbt.log.Errorf("%s: %s", ev.Path, err)
				continue
			}

			if dev == nil {
				fbt.log.Errorf("%s: not found", ev.Path)
				continue
			}

			fbt.log.Debugf("name=%s addr=%s rssi=%d", dev.Properties.Name, dev.Properties.Address, dev.Properties.RSSI)

			go func(ev *adapter.DeviceDiscovered) {
				devData := fbt.handleDiscovery(dev)
				if err != nil {
					fbt.log.Errorf("%s: %s", ev.Path, err)
				}
				watch := false
				for _, reqDev := range watchRequests {
					if devData.Alias == reqDev || devData.Address == reqDev {
						watch = true
						break
					}
				}
				if watch {
					fbt.addMonitor(dev, devData)
				}
			}(ev)
		}
		fbt.log.Debug("Stopped discovery")
	}()
	return nil
}

func (fbt *FreepsBluetooth) handleDiscovery(dev *device.Device1) *DiscoveryData {
	devData := fbt.parseDeviceProperties(dev.Properties)
	ctx := base.NewContext(fbt.log)
	input := freepsgraph.MakeObjectOutput(devData)
	args := map[string]string{"device": devData.Alias, "RSSI": fmt.Sprint(devData.RSSI)}

	deviceTags := []string{"device:" + devData.Alias, "alldevices"}
	if devData.Name != "" {
		deviceTags = append(deviceTags, "nameddevices")
	}

	ns := freepsstore.GetGlobalStore().GetNamespace("_bluetooth")
	ns.SetValue(devData.Address, input, ctx.GetID())
	fbt.ge.ExecuteGraphByTagsExtended(ctx, [][]string{{"bluetooth"}, {"discovered"}, deviceTags}, args, input)

	return devData
}
