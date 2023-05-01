package freepsbluetooth

import "time"

type BluetoothConfig struct {
	Enabled                bool
	AdapterName            string
	DiscoveryDuration      time.Duration
	DiscoveryPauseDuration time.Duration
	ForgetDeviceDuration   time.Duration
	KnownNamespace         string
	DiscoveredNamespace    string
	MonitorsNamespace      string
}

var defaultBluetoothConfig = BluetoothConfig{
	Enabled:                false,
	AdapterName:            "hci0",
	DiscoveryDuration:      time.Minute * 10,
	DiscoveryPauseDuration: time.Minute * 1,
	ForgetDeviceDuration:   time.Hour,
	KnownNamespace:         "_bluetooth_known",
	DiscoveredNamespace:    "_bluetooth_discovered",
	MonitorsNamespace:      "_bluetooth_monitors",
}
