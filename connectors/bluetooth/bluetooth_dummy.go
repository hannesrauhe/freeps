//go:build nobluetooth || windows

package freepsbluetooth

import "github.com/hannesrauhe/freeps/freepsflow"

type Bluetooth freepsflow.DummyOperator
