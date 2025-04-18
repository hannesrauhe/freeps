//go:build !nobluetooth && linux

package freepsbluetooth

import (
	"sync"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
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

	key := devData.Address
	_, ok := m.watchers[key]
	if ok {
		// already monitoring this one
		return false
	}
	ch, err := dev.WatchProperties()
	if err != nil {
		fbt.ctx.GetLogger().Errorf("Cannot watch properties for \"%s\": %s", devData.Alias, err)
		return false
	}
	go fbt.watchProperties(devData, ch)
	m.watchers[key] = deviceEntry{dev: dev, ch: ch}
	return true
}

func (fbt *FreepsBluetooth) getDeviceWatchTags(devData *DiscoveryData) []string {
	return []string{"device:" + devData.Alias, "device:" + devData.Address}
}

func (fbt *FreepsBluetooth) getMonitoredTags() map[string][]string {
	m := fbt.monitors

	m.lck.Lock()
	defer m.lck.Unlock()
	r := map[string][]string{}
	for k, v := range m.watchers {
		deviceTags := []string{"device:" + v.dev.Properties.Alias, "device:" + v.dev.Properties.Address}
		r[k] = deviceTags
	}

	return r
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
	fbt.ctx.GetLogger().Infof("Monitoring device \"%v\"(\"%v\") for changes", devData.Alias, devData.Address)
	alias := devData.Alias
	ns := freepsstore.GetGlobalStore().GetNamespaceNoError(fbt.config.MonitorsNamespace)
	deviceTags := fbt.getDeviceWatchTags(devData)

	debugData := map[string]interface{}{"change": "initial", "devData": devData, "tags": ""}
	ctx := base.CreateContextWithField(fbt.ctx, "component", "bluetooth", "Bluetooth device: "+alias)
	ns.SetValue(devData.Address, base.MakeObjectOutput(debugData), ctx)

	for change := range ch {
		if change == nil {
			continue
		}
		fbt.ctx.GetLogger().Debugf("Changed properties for \"%s\": %s", alias, change)

		ctx := base.CreateContextWithField(fbt.ctx, "component", "bluetooth", "Bluetooth device: "+alias)

		changeTags, err := devData.Update(change.Name, change.Value)
		if err != nil {
			fbt.ctx.GetLogger().Errorf("Cannot update properties for \"%s\": %v", alias, err)
		}
		input := base.MakeObjectOutput(devData)
		args := map[string]string{"device": alias, "address": devData.Address, "change": change.Name}
		taggroups := [][]string{{"bluetooth"}, deviceTags, changeTags}
		fbt.ge.ExecuteFlowByTagsExtended(ctx, taggroups, base.NewFunctionArguments(args), input)

		debugData := map[string]interface{}{"change": change, "devData": devData, "tags": taggroups}
		ns.SetValue(devData.Address, base.MakeObjectOutput(debugData), ctx)
	}
	fbt.ctx.GetLogger().Infof("Stop monitoring \"%s\"(\"%v\") for changes", alias, devData.Address)
}
