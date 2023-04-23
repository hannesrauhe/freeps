//go:build linux

package freepsgraph

import (
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/disk"
	"github.com/mackerelio/go-osstat/loadavg"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/mackerelio/go-osstat/network"
	"github.com/mackerelio/go-osstat/uptime"
)

func (o *OpSystem) Stats(ctx *base.Context, fn string, args map[string]string, input *OperatorIO) *OperatorIO {
	var s interface{}
	var err error
	switch args["statType"] {
	case "cpu":
		s, err = cpu.Get()
	case "disk":
		s, err = disk.Get()
	case "loadavg":
		s, err = loadavg.Get()
	case "memory":
		s, err = memory.Get()
	case "network":
		s, err = network.Get()
	case "uptime":
		s, err = uptime.Get()
	default:
		return MakeOutputError(http.StatusBadRequest, "Stats only available on Linux")
	}
	if err != nil {
		MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	return MakeObjectOutput(s)
}
