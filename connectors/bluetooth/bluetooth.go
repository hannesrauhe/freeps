package freepsbluetooth

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	"context"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/api/beacon"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/sirupsen/logrus"
	eddystone "github.com/suapapa/go_eddystone"
)

// FreepsBluetooth provides options to scan for bt devices and execute operations based on that
type FreepsBluetooth struct {
	log    logrus.FieldLogger
	ge     *freepsgraph.GraphEngine
	cancel context.CancelFunc
}

// NewBTWatcher creates a new BT watcher
func NewBTWatcher(logger logrus.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) (*FreepsBluetooth, error) {
	fbt := &FreepsBluetooth{log: logger.WithField("component", "bluetooth"), ge: ge}
	err := fbt.run("hci0", false)
	if err != nil {
		api.Exit()
	}
	return fbt, err
}

// Shutdown bluetooth scan
func (fbt *FreepsBluetooth) Shutdown() {
	fbt.cancel()
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

	fbt.log.Debug("Start discovery")
	discovery, cancel, err := api.Discover(a, nil)
	if err != nil {
		return err
	}
	fbt.cancel = cancel

	go func() {
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
	}()

	return nil
}

func (fbt *FreepsBluetooth) handleBeacon(dev *device.Device1) error {
	ctx := base.NewContext(fbt.log)
	input := freepsgraph.MakeObjectOutput(dev)
	args := map[string]string{"device": dev.Properties.Alias, "RSSI": fmt.Sprint(dev.Properties.RSSI)}

	freepsstore.GetGlobalStore().GetNamespace("_bluetooth").SetValue(dev.Properties.Alias, input, ctx.GetID())

	tags := []string{"bluetooth", "device:" + dev.Properties.Alias}
	fbt.ge.ExecuteGraphByTags(ctx, tags, args, input)
	tags = []string{"bluetooth", "alldevices"}
	fbt.ge.ExecuteGraphByTags(ctx, tags, args, input)

	// TODO(HR): not sure yet if I actually need the rest:

	b, err := beacon.NewBeacon(dev)
	if err != nil {
		return err
	}

	beaconUpdated, err := b.WatchDeviceChanges(context.Background())
	if err != nil {
		return err
	}

	isBeacon := <-beaconUpdated
	if !isBeacon {
		return nil
	}

	name := b.Device.Properties.Alias
	if name == "" {
		name = b.Device.Properties.Name
	}

	fbt.log.Infof("Found beacon %s %s", b.Type, name)

	if b.IsEddystone() {
		ed := b.GetEddystone()
		switch ed.Frame {
		case eddystone.UID:
			fbt.log.Debugf(
				"Eddystone UID %s instance %s (%ddbi)",
				ed.UID,
				ed.InstanceUID,
				ed.CalibratedTxPower,
			)
			break
		case eddystone.TLM:
			fbt.log.Debugf(
				"Eddystone TLM temp:%.0f batt:%d last reboot:%d advertising pdu:%d (%ddbi)",
				ed.TLMTemperature,
				ed.TLMBatteryVoltage,
				ed.TLMLastRebootedTime,
				ed.TLMAdvertisingPDU,
				ed.CalibratedTxPower,
			)
			break
		case eddystone.URL:
			fbt.log.Debugf(
				"Eddystone URL %s (%ddbi)",
				ed.URL,
				ed.CalibratedTxPower,
			)
			break
		}

	}
	if b.IsIBeacon() {
		ibeacon := b.GetIBeacon()
		fbt.log.Debugf(
			"IBeacon %s (%ddbi) (major=%d minor=%d)",
			ibeacon.ProximityUUID,
			ibeacon.MeasuredPower,
			ibeacon.Major,
			ibeacon.Minor,
		)
	}

	return nil
}
