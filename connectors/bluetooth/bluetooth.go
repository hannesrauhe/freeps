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
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/sirupsen/logrus"
)

var btwatcher *FreepsBluetooth

// TODO(HR): config
var discoveryTime = time.Minute * 10
var discoveryPauseTime = time.Minute * 1
var forgetDeviceTime = time.Hour

// FreepsBluetooth provides options to scan for bt devices and execute operations based on that
type FreepsBluetooth struct {
	discoInitLock      sync.Mutex
	cancel             context.CancelFunc
	shuttingDown       bool
	nextIterationTimer *time.Timer
	log                logrus.FieldLogger
	ge                 *freepsgraph.GraphEngine
}

// NewBTWatcher creates a new BT watcher
func NewBTWatcher(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*FreepsBluetooth, error) {
	btwatcher = &FreepsBluetooth{log: logger.WithField("component", "bluetooth"), shuttingDown: false, ge: ge}

	err := btwatcher.StartSupscription()
	if err != nil {
		btwatcher = nil
		api.Exit()
	}
	return btwatcher, err
}

// StartSupscription starts a discovery process
func (fbt *FreepsBluetooth) StartSupscription() error {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if btwatcher.shuttingDown {
		return nil
	}

	if fbt.cancel != nil {
		return nil
	}
	err := btwatcher.run("hci0", false)
	if err != nil {
		return err
	}
	fbt.nextIterationTimer = time.AfterFunc(discoveryTime, fbt.StopSupscription)
	return err
}

// StopSupscription starts a discovery process
func (fbt *FreepsBluetooth) StopSupscription() {
	fbt.discoInitLock.Lock()
	defer fbt.discoInitLock.Unlock()
	if btwatcher.shuttingDown {
		return
	}

	if fbt.cancel == nil {
		return
	}

	fbt.log.Debug("Stop discovery")
	fbt.cancel()
	fbt.cancel = nil
	fbt.nextIterationTimer = time.AfterFunc(discoveryPauseTime, func() {
		fbt.StartSupscription()
	})
}

// Shutdown bluetooth scan
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

	//TODO(HR): wait here?
	api.Exit()
}

func (fbt *FreepsBluetooth) watchProperties(devData *DiscoveryData, ch chan *bluez.PropertyChanged) {
	alias := devData.Alias
	ns := freepsstore.GetGlobalStore().GetNamespace("_bluetooth")
	deviceTags := []string{"device:" + alias, "device:" + devData.Address}
	for change := range ch {
		fbt.log.Debugf("Changed properties for \"%s\": %s", alias, change)

		ctx := base.NewContext(fbt.log)
		ns.SetValue("CHANGED: "+alias, freepsgraph.MakeObjectOutput(change), ctx.GetID())

		changeTags, err := devData.Update(change.Name, change.Value)
		if err != nil {
			fbt.log.Errorf("Cannot update properties for \"%s\": %v", alias, err)
		}
		input := freepsgraph.MakeObjectOutput(devData)
		args := map[string]string{"device": alias, "change": change.Name}
		fbt.ge.ExecuteGraphByTagsExtended(ctx, [][]string{{"bluetooth"}, deviceTags, changeTags}, args, input)
	}
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
	freepsstore.GetGlobalStore().GetNamespace("_bluetooth").DeleteOlder(forgetDeviceTime)

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
					fbt.log.Infof("Monitoring Device %v for changes", devData.Alias)
					ch, err := dev.WatchProperties()
					if err != nil {
						fbt.log.Errorf("Cannot watch properties for \"%s\": %s", devData.Alias, err)
						return
					}
					go fbt.watchProperties(devData, ch)
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
