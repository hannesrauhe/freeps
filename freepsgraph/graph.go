package freepsgraph

import (
	"log"

	"github.com/hannesrauhe/freeps/utils"
)

var ROOT_SYMBOL = "_"

//GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs         map[string]GraphDesc
	GraphsFromURL  []string
	GraphsFromFile []string
}

var DefaultGraphEngineConfig = GraphEngineConfig{GraphsFromFile: []string{"graphs.json"}}

//GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name          string
	Operator      string
	Function      string
	Arguments     map[string]string `json:",omitempty"`
	InputFrom     string            `json:",omitempty"`
	ArgumentsFrom string            `json:",omitempty"`
}

//GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	OutputFrom string
	Operations []GraphOperationDesc
}

//Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

//GraphEngine holds all available graphs and operators
type GraphEngine struct {
	configGraphs    map[string]GraphDesc
	externalGraphs  map[string]GraphDesc
	temporaryGraphs map[string]GraphDesc
	operators       map[string]FreepsOperator
}

//NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader) *GraphEngine {
	ge := &GraphEngine{configGraphs: make(map[string]GraphDesc), externalGraphs: make(map[string]GraphDesc), temporaryGraphs: make(map[string]GraphDesc)}
	config := DefaultGraphEngineConfig
	err := cr.ReadSectionWithDefaults("graphs", &config)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	for _, fName := range config.GraphsFromFile {
		err = cr.ReadObjectFromFile(&ge.externalGraphs, fName)
		if err != nil {
			log.Fatal(err)
		}
	}

	ge.operators = make(map[string]FreepsOperator)
	ge.operators["template"] = NewTemplateOperator(cr)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["curl"] = &OpCurl{}

	return ge
}

//ExecuteOperatorByName executes an operator directly
func (ge *GraphEngine) ExecuteOperatorByName(opName string, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g := &Graph{engine: ge}
	g.opOutputs = make(map[string]*OperatorIO)
	g.opOutputs[ROOT_SYMBOL] = mainInput
	return g.executeOperation(&GraphOperationDesc{Name: "#0", Operator: opName, Function: fn, InputFrom: ROOT_SYMBOL}, mainArgs)
}

//ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(graphName string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	gd, exists := ge.GetGraphDesc(graphName)
	if exists {
		g := &Graph{engine: ge, desc: gd}
		g.opOutputs = make(map[string]*OperatorIO)
		g.opOutputs[ROOT_SYMBOL] = mainInput
		return g.execute(mainArgs)
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", graphName)
}

func (ge *GraphEngine) GetGraphDesc(graphName string) (*GraphDesc, bool) {
	gd, exists := ge.configGraphs[graphName]
	if exists {
		return &gd, exists
	}
	gd, exists = ge.externalGraphs[graphName]
	if exists {
		return &gd, exists
	}
	gd, exists = ge.temporaryGraphs[graphName]
	if exists {
		return &gd, exists
	}
	return nil, false
}

func (g *Graph) executeOperation(opDesc *GraphOperationDesc, mainArgs map[string]string) *OperatorIO {
	input := MakeEmptyOutput()
	if opDesc.InputFrom != "" {
		var exists bool
		input, exists = g.opOutputs[opDesc.InputFrom]
		if !exists {
			return MakeOutputError(404, "Output of \"%s\" cannot be used as input for \"%v\", because there is no such output", opDesc.InputFrom, opDesc.Name)
		}
	}
	combinedArgs := make(map[string]string)
	if opDesc.Arguments != nil {
		for k, v := range opDesc.Arguments {
			combinedArgs[k] = v
		}
	}
	if opDesc.ArgumentsFrom == ROOT_SYMBOL {
		for k, v := range mainArgs {
			combinedArgs[k] = v
		}
	} else if opDesc.ArgumentsFrom != "" {
		outputToBeArgs, exists := g.opOutputs[opDesc.ArgumentsFrom]
		if !exists {
			return MakeOutputError(404, "Output of \"%s\" cannot be used as arguments, because there is no such output", opDesc.ArgumentsFrom)
		}
		collectedArgs, err := outputToBeArgs.GetMap()
		if err != nil {
			return MakeOutputError(500, "Output of \"%s\" cannot be used as arguments, because it's of type \"%s\"", opDesc.ArgumentsFrom, outputToBeArgs.OutputType)
		}
		for k, v := range collectedArgs {
			combinedArgs[k] = v
		}
	}

	op, exists := g.engine.operators[opDesc.Operator]
	if exists {
		return op.Execute(opDesc.Function, combinedArgs, input)
	}
	return MakeOutputError(404, "No operator with name \"%s\" found", opDesc.Operator)
}

func (g *Graph) execute(mainArgs map[string]string) *OperatorIO {
	for _, operation := range g.desc.Operations {
		_, exist := g.opOutputs[operation.Name]
		if exist {
			return MakeOutputError(404, "Multiple operations with name \"%s\" found", operation.Name)
		}
		output := g.executeOperation(&operation, mainArgs)
		g.opOutputs[operation.Name] = output
		if output.IsError() {
			return output
		}
	}
	if g.desc.OutputFrom == "" {
		lastOperation := g.desc.Operations[len(g.desc.Operations)-1]
		return g.opOutputs[lastOperation.Name]
	}
	return g.opOutputs[g.desc.OutputFrom]
}
