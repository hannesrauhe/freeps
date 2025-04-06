package freepsflow

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hannesrauhe/freeps/base"
)

//go:embed embedded_flows/*
var embeddedFlows embed.FS

// GetFlowDir returns the directory where the flows are stored
func (ge *FlowEngine) GetFlowDir() string {
	d := ge.cr.GetConfigDir() + "/graphs"
	// create directory if it does not exist
	if _, err := os.Stat(d); os.IsNotExist(err) {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			panic("could not create flow directory: " + err.Error())
		}
	}
	return d
}

func (ge *FlowEngine) loadStoredAndEmbeddedFlows(ctx *base.Context) error {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()

	embeddedList, err := embeddedFlows.ReadDir("embedded_flows")
	if err != nil {
		panic("could not read embedded flows: " + err.Error())
	}

	for _, e := range embeddedList {
		if e.IsDir() {
			continue
		}
		gb, err := embeddedFlows.ReadFile("embedded_flows/" + e.Name())
		if err != nil {
			panic("could not read embedded flow " + e.Name() + ": " + err.Error())
		}
		gd := FlowDesc{}
		err = json.Unmarshal(gb, &gd)
		if err != nil {
			panic("embedded flow " + e.Name() + " is invalid: " + err.Error())
		}
		gd.Source = "embedded"

		err = ge.AddFlowUnderLock(ctx, e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			ctx.GetLogger().Warnf("Could not load embedded flow \"%v\": %v", e.Name(), err)
		}
	}

	storedList, err := os.ReadDir(ge.GetFlowDir())
	if err != nil {
		panic("could not read stored flows: " + err.Error())
	}
	for _, e := range storedList {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		gb, err := os.ReadFile(ge.GetFlowDir() + "/" + e.Name())
		if err != nil {
			panic("could not read stored flow " + e.Name() + ": " + err.Error())
		}
		gd := FlowDesc{}
		err = json.Unmarshal(gb, &gd)
		if err != nil {
			ctx.GetLogger().Warnf("Could not load stored flow \"%v\": %v", e.Name(), err)
		}

		err = ge.AddFlowUnderLock(ctx, e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			ctx.GetLogger().Warnf("Could not load stored flow \"%v\": %v", e.Name(), err)
		}
	}

	return nil
}
