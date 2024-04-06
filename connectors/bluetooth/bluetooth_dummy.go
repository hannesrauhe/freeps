//go:build nobluetooth || windows

package freepsbluetooth

import "github.com/hannesrauhe/freeps/freepsgraph"

type Bluetooth freepsgraph.DummyOperator
