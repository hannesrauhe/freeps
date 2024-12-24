package freepsflow

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hannesrauhe/freeps/base"
	log "github.com/sirupsen/logrus"
)

//go:embed embedded_flows/*
var embeddedFlows embed.FS

// GetFlowDir returns the directory where the flows are stored
func (ge *FlowEngine) GetFlowDir() string {
	d := ge.cr.GetConfigDir() + "/flows"
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

		err = ge.addFlowUnderLock(ctx, e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			log.Warnf("Could not load embeddedd flow \"%v\": %v", e.Name(), err)
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
			log.Warnf("Could not load stored flow \"%v\": %v", e.Name(), err)
		}

		err = ge.addFlowUnderLock(ctx, e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			log.Warnf("Could not load stored flow \"%v\": %v", e.Name(), err)
		}
	}

	return nil
}

// loadExternalFlows loads flows from the configured URLs and files - should probably be deprecated and moved into an operator
func (ge *FlowEngine) loadExternalFlows(ctx *base.Context) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()

	config := ge.ReadConfig()
	var err error

	for _, fURL := range config.FlowsFromURL {
		newFlows := make(map[string]FlowDesc)
		err = ge.cr.ReadObjectFromURL(&newFlows, fURL)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fURL, err)
		}
		ge.addExternalFlowsWithSource(ctx, newFlows, "url: "+fURL)
	}
	config.FlowsFromURL = []string{}
	for _, fName := range config.FlowsFromFile {
		newFlows := make(map[string]FlowDesc)
		err = ge.cr.ReadObjectFromFile(&newFlows, fName)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fName, err)
		}
		ge.addExternalFlowsWithSource(ctx, newFlows, "file: "+fName)
	}
	config.FlowsFromFile = []string{}
	err = ge.cr.WriteSection("flows", &config, true)
	if err != nil {

	}
}

func (ge *FlowEngine) addExternalFlowsWithSource(ctx *base.Context, src map[string]FlowDesc, srcName string) {
	for k, v := range src {
		v.Source = srcName
		err := ge.addFlowUnderLock(ctx, k, v, true, true)
		if err != nil {
			log.Errorf("Skipping flow %v from %v, because: %v", k, srcName, err)
		}
	}
}
