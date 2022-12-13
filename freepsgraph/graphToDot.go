package freepsgraph

import (
	"bytes"
	"strings"

	"github.com/capossele/GoGraphviz/graphviz"
	"github.com/hannesrauhe/freeps/utils"
)

func (g *Graph) toDot(ctx *utils.Context, G *graphviz.Graph, nameIDMap map[string]int, mainInputID int) {
	for _, v := range g.desc.Operations {
		nodename := strings.Join([]string{g.name, v.Name}, ".")
		for true {
			if _, ok := nameIDMap[nodename]; ok {
				nodename = nodename + "."
				continue
			}
			break
		}
		nameIDMap[nodename] = G.AddNode(nodename)
		if v.InputFrom != "" {
			if v.InputFrom == "_" {
				G.AddEdge(mainInputID, nameIDMap[nodename], "input")
			} else {
				G.AddEdge(nameIDMap[v.InputFrom], nameIDMap[nodename], "input")
			}
		}
		if v.ExecuteOnFailOf != "" {
			G.AddEdge(nameIDMap[v.ExecuteOnFailOf], nameIDMap[nodename], "fail")
		}
		if v.ArgumentsFrom != "" {
			if v.ArgumentsFrom == "_" {
				G.AddEdge(mainInputID, nameIDMap[nodename], "args")
			} else {
				G.AddEdge(nameIDMap[v.ArgumentsFrom], nameIDMap[nodename], "args")
			}
		}
		if !v.IgnoreMainArgs {
			G.AddEdge(mainInputID, nameIDMap[nodename], "args")
		}
		if v.Operator == "graph" {
			sg, _ := g.engine.prepareGraphExecution(ctx, v.Function, false)
			if sg != nil {
				sg.toDot(ctx, G, nameIDMap, nameIDMap[nodename])
			}
		}
	}
	if g.desc.OutputFrom != "" {
		G.NodeAttribute(nameIDMap[g.desc.OutputFrom], "output", "output")
	}
}

// ToDot creates the Graphviz/dot represantion of a graph
func (g *Graph) ToDot(ctx *utils.Context) []byte {
	nameIDMap := map[string]int{}
	G := graphviz.Graph{}
	G.MakeDirected()
	G.DrawMultipleEdges()
	nameIDMap["_"] = G.AddNode("mainInput")
	g.toDot(ctx, &G, nameIDMap, nameIDMap["_"])
	buf := new(bytes.Buffer)
	G.GenerateDOT(buf)
	return buf.Bytes()
}
