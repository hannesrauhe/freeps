package freepsgraph

import (
	"embed"
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"
)

//go:embed embedded_graphs/*
var embeddedGraphs embed.FS

// GetGraphDir returns the directory where the graphs are stored
func (ge *GraphEngine) GetGraphDir() string {
	d := ge.cr.GetConfigDir() + "/graphs"
	// create directory if it does not exist
	if _, err := os.Stat(d); os.IsNotExist(err) {
		err = os.MkdirAll(d, 0755)
		if err != nil {
			panic("could not create graph directory: " + err.Error())
		}
	}
	return d
}

func (ge *GraphEngine) loadStoredAndEmbeddedGraphs() error {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	embeddedList, err := embeddedGraphs.ReadDir("embedded_graphs")
	if err != nil {
		panic("could not read embedded graphs: " + err.Error())
	}

	for _, e := range embeddedList {
		if e.IsDir() {
			continue
		}
		gb, err := embeddedGraphs.ReadFile("embedded_graphs/" + e.Name())
		if err != nil {
			panic("could not read embedded graph " + e.Name() + ": " + err.Error())
		}
		gd := GraphDesc{}
		err = json.Unmarshal(gb, &gd)
		if err != nil {
			panic("embedded graph " + e.Name() + " is invalid: " + err.Error())
		}
		gd.Source = "embedded"

		err = ge.addGraphUnderLock(e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			log.Warnf("Could not load embeddedd graph \"%v\": %v", e.Name(), err)
		}
	}

	storedList, err := os.ReadDir(ge.GetGraphDir())
	if err != nil {
		panic("could not read stored graphs: " + err.Error())
	}
	for _, e := range storedList {
		if e.IsDir() {
			continue
		}
		gb, err := os.ReadFile(ge.GetGraphDir() + "/" + e.Name())
		if err != nil {
			panic("could not read stored graph " + e.Name() + ": " + err.Error())
		}
		gd := GraphDesc{}
		err = json.Unmarshal(gb, &gd)
		if err != nil {
			log.Warnf("Could not load stored graph \"%v\": %v", e.Name(), err)
		}

		err = ge.addGraphUnderLock(e.Name()[:len(e.Name())-5], gd, false, false)
		if err != nil {
			log.Warnf("Could not load stored graph \"%v\": %v", e.Name(), err)
		}
	}

	return nil
}

// loadExternalGraphs loads graphs from the configured URLs and files - should probably be deprecated and moved into an operator
func (ge *GraphEngine) loadExternalGraphs() {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	config := ge.ReadConfig()
	var err error

	for _, fURL := range config.GraphsFromURL {
		newGraphs := make(map[string]GraphDesc)
		err = ge.cr.ReadObjectFromURL(&newGraphs, fURL)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fURL, err)
		}
		ge.addExternalGraphsWithSource(newGraphs, "url: "+fURL)
	}
	config.GraphsFromURL = []string{}
	for _, fName := range config.GraphsFromFile {
		newGraphs := make(map[string]GraphDesc)
		err = ge.cr.ReadObjectFromFile(&newGraphs, fName)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fName, err)
		}
		ge.addExternalGraphsWithSource(newGraphs, "file: "+fName)
	}
	config.GraphsFromFile = []string{}
	ge.cr.WriteSection("graphs", &config, true)
}

func (ge *GraphEngine) addExternalGraphsWithSource(src map[string]GraphDesc, srcName string) {
	for k, v := range src {
		v.Source = srcName
		err := ge.addGraphUnderLock(k, v, true, true)
		if err != nil {
			log.Errorf("Skipping graph %v from %v, because: %v", k, srcName, err)
		}
	}
}
