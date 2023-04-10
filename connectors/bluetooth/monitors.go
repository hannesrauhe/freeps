//go:build !nobluetooth

package freepsbluetooth

import (
	"sync"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

type deviceEntry struct {
	dev *device.Device1
	ch  chan *bluez.PropertyChanged
}

type monitors struct {
	lck      sync.Mutex
	watchers map[string]deviceEntry
}

func (fbt *FreepsBluetooth) addMonitor(dev *device.Device1, devData *DiscoveryData) bool {
	m := fbt.monitors

	m.lck.Lock()
	defer m.lck.Unlock()

	key := devData.Alias
	_, ok := m.watchers[key]
	if ok {
		// already monitoring this one
		return false
	}
	ch, err := dev.WatchProperties()
	if err != nil {
		fbt.log.Errorf("Cannot watch properties for \"%s\": %s", devData.Alias, err)
		return false
	}
	go fbt.watchProperties(devData, ch)
	m.watchers[key] = deviceEntry{dev: dev, ch: ch}
	return true
}

func (fbt *FreepsBluetooth) deleteMonitor(key string) bool {
	m := fbt.monitors

	m.lck.Lock()
	defer m.lck.Unlock()
	dEntry, ok := m.watchers[key]
	if !ok {
		return false
	}
	dEntry.dev.UnwatchProperties(dEntry.ch)
	delete(m.watchers, key)
	return true
}

func (fbt *FreepsBluetooth) deleteAllMonitors() {
	m := fbt.monitors

	m.lck.Lock()
	defer m.lck.Unlock()
	for _, dEntry := range m.watchers {
		dEntry.dev.UnwatchProperties(dEntry.ch)
	}
	m.watchers = map[string]deviceEntry{}
}

func (fbt *FreepsBluetooth) watchProperties(devData *DiscoveryData, ch chan *bluez.PropertyChanged) {
	fbt.log.Infof("Monitoring device \"%v\" for changes", devData.Alias)
	alias := devData.Alias
	ns := freepsstore.GetGlobalStore().GetNamespace("_bluetooth")
	deviceTags := []string{"device:" + alias, "device:" + devData.Address}
	for change := range ch {
		if change == nil {
			continue
		}
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
	fbt.log.Infof("Stop monitoring \"%s\" for changes", alias)
}
