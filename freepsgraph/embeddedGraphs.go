package freepsgraph

import (
	"embed"
	"encoding/json"
)

//go:embed embedded_graphs/*
var embeddedGraphs embed.FS

func (ge *GraphEngine) LoadEmbeddedGraphs() error {
	ftlist, _ := embeddedGraphs.ReadDir("embedded_graphs")
	for _, e := range ftlist {
		if e.IsDir() {
			continue
		}
		gb, err := embeddedGraphs.ReadFile("embedded_graphs/" + e.Name())
		if err != nil {
			return err
		}
		gd := &GraphDesc{}
		err = json.Unmarshal(gb, gd)
		if err != nil {
			return err
		}
		err = ge.AddTemporaryGraph(e.Name()[:len(e.Name())-5], gd, "embedded")
		if err != nil {
			return err
		}
	}
	return nil
}
