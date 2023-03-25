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
	fbt.nextIterationTimer = time.AfterFunc(time.Minute*2, fbt.StopSupscription)
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
	fbt.nextIterationTimer = time.AfterFunc(time.Minute*5, func() {
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
	freepsstore.GetGlobalStore().GetNamespace("_bluetooth").DeleteOlder(time.Hour)

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

			go func(ev *adapter.DeviceDiscovered) {
				err = fbt.handleBeacon(dev)
				if err != nil {
					fbt.log.Errorf("%s: %s", ev.Path, err)
				}
			}(ev)
		}
		fbt.log.Debug("Stopped discovery")
	}()
	return nil
}

func (fbt *FreepsBluetooth) handleBeacon(dev *device.Device1) error {
	ctx := base.NewContext(fbt.log)
	input := freepsgraph.MakeObjectOutput(dev.Properties)
	args := map[string]string{"device": dev.Properties.Alias, "RSSI": fmt.Sprint(dev.Properties.RSSI)}

	freepsstore.GetGlobalStore().GetNamespace("_bluetooth").SetValue(dev.Properties.Alias, input, ctx.GetID())

	tags := []string{"device:" + dev.Properties.Alias, "alldevices"}
	if dev.Properties.AddressType == "public" {
		tags = append(tags, "publicdevices")
	}
	if dev.Properties.Name != "" {
		tags = append(tags, "nameddevices")
	}
	fbt.ge.ExecuteGraphByTagsExtended(ctx, []string{"bluetooth"}, tags, args, input)

	for k, v := range dev.Properties.ServiceData {
		args := map[string]string{"device": dev.Properties.Alias, "RSSI": fmt.Sprint(dev.Properties.RSSI), "service": k}
		fbt.ge.ExecuteGraphByTagsExtended(ctx, []string{"bluetooth", "service"}, []string{k, "allservices"}, args, freepsgraph.MakeObjectOutput(v))
	}

	return nil
}
