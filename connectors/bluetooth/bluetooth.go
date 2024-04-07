//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"fmt"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"

	"context"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/sirupsen/logrus"
)

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

// StartDiscovery starts the process of discovery new bluetooth devices
func (fbt *FreepsBluetooth) StartDiscovery() error {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if fbt.shuttingDown {
		return nil
	}

	if fbt.cancel != nil {
		return nil
	}
	err := fbt.run(fbt.config.AdapterName, false)
	if err != nil {
		return err
	}
	fbt.nextIterationTimer = time.AfterFunc(fbt.config.DiscoveryDuration, func() {
		fbt.StopDiscovery(false)
	})
	return err
}

func (fbt *FreepsBluetooth) run(adapterID string, flushDevices bool) error {
	a, err := adapter.GetAdapter(adapterID)
	if err != nil {
		return err
	}

	if flushDevices {
		fbt.log.Debug("Flush cached devices")
		err = a.FlushDevices()
		if err != nil {
			return err
		}
	} else {
		fbt.log.Debug("Flush cached devices skipped")
	}

	devices, err := a.GetDevices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		go fbt.handleNewDevice(dev, false)
	}

	freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.KnownNamespace).DeleteOlder(fbt.config.ForgetDeviceDuration)
	freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.DiscoveredNamespace).DeleteOlder(fbt.config.ForgetDeviceDuration)
	freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.MonitorsNamespace).DeleteOlder(fbt.config.ForgetDeviceDuration)

	discovery, cancel, err := api.Discover(a, nil)
	if err != nil {
		return err
	}
	fbt.cancel = cancel

	go func() {
		fbt.log.Debug("Started discovery")
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

			go fbt.handleNewDevice(dev, true)
		}
		fbt.log.Debug("Stopped discovery")
	}()
	return nil
}

func (fbt *FreepsBluetooth) handleNewDevice(dev *device.Device1, freshDiscovery bool) *DiscoveryData {
	devData := fbt.parseDeviceProperties(dev.Properties)
	ctx := base.NewContext(fbt.log, "Bluetooth device: "+devData.Alias)
	input := base.MakeObjectOutput(devData)
	args := map[string]string{"device": devData.Alias, "address": devData.Address, "RSSI": fmt.Sprint(devData.RSSI)}

	deviceTags := []string{"device:" + devData.Alias, "address:" + devData.Address, "alldevices"}
	if devData.Name != "" {
		deviceTags = append(deviceTags, "nameddevices")
	}

	if freshDiscovery {
		ns := freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.DiscoveredNamespace)
		ns.SetValue(devData.Address, input, ctx)
		fbt.ge.ExecuteGraphByTagsExtended(ctx, [][]string{{"bluetooth"}, {"discovered"}, deviceTags}, base.NewFunctionArguments(args), input)
	} else {
		ns := freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.KnownNamespace)
		ns.SetValue(devData.Address, input, ctx)
	}

	// start watcher if requested
	watchDeviceTags := fbt.getDeviceWatchTags(devData)
	if len(fbt.ge.GetGraphDescByTagExtended([]string{"bluetooth"}, watchDeviceTags)) > 0 {
		fbt.addMonitor(dev, devData)
	}

	return devData
}

// StopDiscovery stops bluetooth discovery and schedules the next discovery process
func (fbt *FreepsBluetooth) StopDiscovery(restartImmediately bool) {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if fbt.shuttingDown {
		return
	}

	if fbt.cancel == nil {
		return
	}

	fbt.log.Infof("Stopping discovery, immediate restart: %v", restartImmediately)
	fbt.cancel()
	fbt.cancel = nil

	// remove monitors that have no graphs anymore
	for w, deviceTags := range fbt.getMonitoredTags() {
		if len(fbt.ge.GetGraphDescByTagExtended([]string{"bluetooth"}, deviceTags)) == 0 {
			fbt.deleteMonitor(w)
		}
	}
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
